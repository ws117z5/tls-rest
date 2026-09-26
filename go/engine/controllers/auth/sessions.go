package auth

import (
	"context"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"tls-rest/go/engine/controllers/config"
	"tls-rest/go/engine/controllers/functions"
	"tls-rest/go/engine/controllers/httpx"

	"tls-rest/go/engine/controllers/db/cache"
)

type ContextKey string

// SESSION_KEY is the context key under which the request's *cache.Session is
// stored. It aliases cache.SessionKey so packages that cannot import auth (the
// module engine) can read the same session via cache.SessionFromContext.
var SESSION_KEY = cache.SessionKey

// rightsEpoch: bumped on any rights change; seeded from start time (not 0) so a Redis-persisted session can't coincidentally match it after a restart.
var rightsEpoch = time.Now().Unix()

// CurrentRightsEpoch returns the current global rights epoch.
func CurrentRightsEpoch() int64 { return atomic.LoadInt64(&rightsEpoch) }

// BumpRightsEpoch invalidates cached session rights everywhere. Call it after any
// change to users, groups, or rights so active sessions pick up the new rights on
// their next request.
func BumpRightsEpoch() { atomic.AddInt64(&rightsEpoch, 1) }

// fillSessionRights resolves and attaches the user's per-module mode rights,
// access level and admin status to the session, in a single place so every code
// path (new session, restored session, anonymous) is populated consistently.
// Anonymous sessions (UserID <= 0) resolve as a member of auth.GuestGroupID
// (module defaults still apply on top) and are never admin.
func fillSessionRights(ctx context.Context, s *cache.Session) {
	s.ModuleModes, s.FieldRights, s.SpecialRights, s.FilterFieldRights, s.AccessLevel, s.IsAdmin = resolveSessionRights(ctx, s.UserID)
	s.RightsEpoch = CurrentRightsEpoch()
	// Resolve config alongside rights so a freshly created/refreshed session has
	// both populated.
	fillSessionConfig(ctx, s)
}

// fillSessionConfig resolves and caches the user's effective config on the
// session, stamping the config epoch it was resolved at.
func fillSessionConfig(ctx context.Context, s *cache.Session) {
	s.Config = config.Resolve(ctx, s.UserID)
	s.ConfigEpoch = config.CurrentConfigEpoch()
}

// rightsStale reports whether a stored session's cached rights predate the
// current epoch and must be re-resolved.
func rightsStale(s *cache.Session) bool {
	return s.RightsEpoch != CurrentRightsEpoch()
}

// configStale reports whether a stored session's cached config predates the
// current config epoch.
func configStale(s *cache.Session) bool {
	return s.ConfigEpoch != config.CurrentConfigEpoch()
}

// Checks session and fills cache with session data
// should fill session user and it's rights
// If the session does not exist, it creates a new one
func ManageSession(w http.ResponseWriter, r *http.Request) *cache.Session {
	// Non-web (mobile) clients authenticate with a bearer token instead of the
	// session cookie. When present it fully determines the session, and no cookie
	// is set. The cookie flow below is unchanged for web clients.
	if tok, ok := tokenFromHeader(r); ok {
		return manageTokenSession(r.Context(), tok)
	}

	ip := httpx.ClientIP(r)
	ua := r.UserAgent()

	// Only a live session the server itself issued is honoured; a missing, unknown (client-chosen) or expired id gets a fresh server-minted one.
	if cookie, err := r.Cookie("X-Session-ID"); err == nil {
		if stored, e := cache.SessionCacheInstance.Get(cookie.Value); e == nil && stored != nil && stored.Expire.After(time.Now()) {
			stored.UserAgent = ua
			stored.IP = ip
			stored.Expire = time.Now().Add(30 * 24 * time.Hour) // sliding 30 days
			stored.LastAccess = time.Now()

			// Rights are cached in the session; only re-resolve when a
			// user/group/rights change bumped the global epoch. This avoids
			// hitting the DB for rights on every request.
			if rightsStale(stored) {
				fillSessionRights(r.Context(), stored)
			} else if configStale(stored) {
				fillSessionConfig(r.Context(), stored)
			}

			cache.SessionCacheInstance.Set(cookie.Value, *stored)
			return stored
		}
	}

	hash, _ := functions.GetRandomHash(16)
	expire := time.Now().Add(30 * 24 * time.Hour)
	ci := cache.Session{UserAgent: ua, IP: ip, Expire: expire, LastAccess: time.Now()}
	http.SetCookie(w, sessionCookie(r, hash, expire))
	fillSessionRights(r.Context(), &ci)
	cache.SessionCacheInstance.Set(hash, ci)
	return &ci
}

// sessionCookie builds the session cookie: unreadable by scripts, not sent on cross-site subrequests, and HTTPS-only when served over HTTPS.
func sessionCookie(r *http.Request, value string, expire time.Time) *http.Cookie {
	return &http.Cookie{
		Name:     "X-Session-ID",
		Value:    value,
		Expires:  expire,
		Path:     "/",
		HttpOnly: true,
		Secure:   httpx.Scheme(r) == "https",
		SameSite: http.SameSiteLaxMode,
	}
}

// GetSessionID returns the session id from context.
func GetSessionID(ctx context.Context) string {
	val := ctx.Value(SESSION_KEY)
	if sid, ok := val.(string); ok {
		return sid
	}
	return ""
}

// Login establishes an authenticated session for userID under a brand-new
// session id (the previous one is invalidated, so a pre-planted id can't be
// promoted to an authenticated session) and resolves rights/admin status. Used
// by both password login and OAuth callbacks — the one place that actually sets
// UserID on a session.
func Login(w http.ResponseWriter, r *http.Request, userID int, username string) {
	expire := time.Now().Add(30 * 24 * time.Hour)

	hash, _ := functions.GetRandomHash(16)
	http.SetCookie(w, sessionCookie(r, hash, expire))

	if old, err := r.Cookie("X-Session-ID"); err == nil && old.Value != "" {
		RevokeToken(old.Value)
	}

	s := &cache.Session{
		UserAgent:  r.UserAgent(),
		IP:         httpx.ClientIP(r),
		UserID:     userID,
		Username:   username,
		Expire:     expire,
		LastAccess: time.Now(),
	}
	fillSessionRights(r.Context(), s)

	cache.SessionCacheInstance.Set(hash, *s)
}

// Logout clears the authenticated user from the current session, dropping it
// back to anonymous (module defaults, not admin).
func Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("X-Session-ID")
	if err != nil {
		return
	}
	if stored, e := cache.SessionCacheInstance.Get(cookie.Value); e == nil && stored != nil {
		stored.UserID = 0
		stored.Username = ""
		fillSessionRights(r.Context(), stored)
		cache.SessionCacheInstance.Set(cookie.Value, *stored)
	}
}

// --- Non-web (mobile) bearer-token auth ---
//
// A bearer token is just a session key handed to a client that can't use
// cookies. It maps to exactly the same cache-backed session and rights model as
// the web cookie session, so authorization is identical across web and mobile.

// tokenFromHeader returns a bearer token supplied via `Authorization: Bearer
// <token>` (or the `X-Session-Token` header) and whether one was present.
func tokenFromHeader(r *http.Request) (string, bool) {
	if h := r.Header.Get("Authorization"); len(h) >= 7 && strings.EqualFold(h[:7], "Bearer ") {
		if tok := strings.TrimSpace(h[7:]); tok != "" {
			return tok, true
		}
	}
	if tok := strings.TrimSpace(r.Header.Get("X-Session-Token")); tok != "" {
		return tok, true
	}
	return "", false
}

// manageTokenSession resolves the session for a bearer-token request. A valid,
// unexpired token returns its stored session (rights refreshed); an unknown or
// expired token yields a fresh anonymous session. No cookie is ever set.
func manageTokenSession(ctx context.Context, tok string) *cache.Session {
	if stored, err := cache.SessionCacheInstance.Get(tok); err == nil && stored != nil && stored.Expire.After(time.Now()) {
		stored.LastAccess = time.Now()
		if rightsStale(stored) {
			fillSessionRights(ctx, stored)
		} else if configStale(stored) {
			fillSessionConfig(ctx, stored)
		}
		cache.SessionCacheInstance.Set(tok, *stored)
		return stored
	}
	anon := cache.Session{Expire: time.Now().Add(30 * 24 * time.Hour), LastAccess: time.Now()}
	fillSessionRights(ctx, &anon)
	return &anon
}

// IssueToken creates an authenticated, cookie-less session for a non-web client
// and returns its opaque bearer token and expiry. The client sends the token as
// `Authorization: Bearer <token>` on subsequent requests.
func IssueToken(ctx context.Context, userID int, username string) (string, time.Time, error) {
	tok, err := functions.GetRandomHash(32)
	if err != nil {
		return "", time.Time{}, err
	}
	expire := time.Now().Add(30 * 24 * time.Hour)
	s := cache.Session{UserID: userID, Username: username, Expire: expire, LastAccess: time.Now()}
	fillSessionRights(ctx, &s)
	cache.SessionCacheInstance.Set(tok, s)
	return tok, expire, nil
}

// RevokeToken invalidates a bearer token (mobile logout). The cache exposes no
// delete, so the entry is overwritten with an already-expired anonymous session.
func RevokeToken(tok string) {
	if tok == "" {
		return
	}
	cache.SessionCacheInstance.Set(tok, cache.Session{Expire: time.Now().Add(-time.Hour)})
}
