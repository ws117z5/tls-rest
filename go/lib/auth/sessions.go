package auth

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"tls-rest/go/lib"

	"tls-rest/go/lib/db/cache"
)

type ContextKey string

// SESSION_KEY is the context key under which the request's *cache.Session is
// stored. It aliases cache.SessionKey so packages that cannot import auth (the
// module engine) can read the same session via cache.SessionFromContext.
var SESSION_KEY = cache.SessionKey

// fillSessionRights resolves and attaches the user's per-module mode rights,
// access level and admin status to the session, in a single place so every code
// path (new session, restored session, anonymous) is populated consistently.
// Anonymous sessions (UserID <= 0) get module defaults and are never admin.
func fillSessionRights(s *cache.Session) {
	s.ModuleModes = ResolveModuleModeRights(s.UserID)
	s.AccessLevel = ResolveUserAccessLevel(s.UserID)
	s.IsAdmin = ResolveIsAdmin(s.UserID)
}

// Checks session and fills cache with session data
// should fill session user and it's rights
// If the session does not exist, it creates a new one
func ManageSession(w http.ResponseWriter, r *http.Request) *cache.Session {
	cookie, err := r.Cookie("X-Session-ID")

	hash := ""
	expire := time.Now().Add(30 * 24 * time.Hour) // 30 days

	if err != nil {
		hash, _ = lib.GetRandomHash(16)
	} else {
		hash = cookie.Value
	}

	ip := r.Header.Get("X-Forwarded-For")
	if ip == "" {
		host, _, err := net.SplitHostPort(r.RemoteAddr)

		if err != nil {
			ip = r.RemoteAddr // fallback to full RemoteAddr if parsing fails
		} else {
			ip = host
		}
	}

	// Get User-Agent
	ua := r.UserAgent()

	ci := cache.Session{
		UserAgent:  ua,
		IP:         ip,
		Expire:     expire, // 30 days
		LastAccess: time.Now(),
	}

	//if coockie is not set, create a new one
	if err != nil {
		cookie = &http.Cookie{
			Name:    "X-Session-ID",
			Value:   hash,
			Expires: expire,
			Path:    "/",
		}

		http.SetCookie(w, cookie)

		fillSessionRights(&ci)
		cache.SessionCacheInstance.Set(cookie.Value, ci)

		return &ci
	} else {
		// If the cookie exists, we can check its value

		stored, err := cache.SessionCacheInstance.Get(cookie.Value)

		if err != nil {
			// If the session does not exist, create a new one
			fillSessionRights(&ci)
			cache.SessionCacheInstance.Set(hash, ci)
			return &ci
		} else {
			if stored.Expire.Before(time.Now()) {
				// If the session has expired, create a new one
				fmt.Println("Session expired, creating a new one")
			}
			// Update the existing session
			stored.UserAgent = ua
			stored.IP = ip
			stored.Expire = time.Now().Add(30 * 24 * time.Hour) // 30 days
			stored.LastAccess = time.Now()

			// Resolve the user's rights, access level and admin status through the
			// per-mode group model (resolve.go), in one place for every session.
			fillSessionRights(stored)

			cache.SessionCacheInstance.Set(hash, *stored)

			return stored
		}
	}
}

func GetSessionID(ctx context.Context) string {
	val := ctx.Value(SESSION_KEY)
	if sid, ok := val.(string); ok {
		return sid
	}
	return ""
}
