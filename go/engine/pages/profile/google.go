// Google Calendar connection — a separate OAuth consent flow from the app's
// existing "Login with Google" (auth/providers.go), which only requests
// identity scopes and never asks for offline access. This one requests
// calendar.events (read/write events, nothing else) with offline access and
// forced consent, so it always gets a refresh token it can use later, from a
// background job, with nobody watching. Reuses the same Google OAuth client
// (GoogleID/GoogleSecret) as login — add this callback URL to that client's
// authorized redirect URIs in Google Cloud Console, and enable the Calendar
// API on that project. No second OAuth client needed.
//
// This app only ever pushes its own events out (see googlepush.go, called
// from events.go's AfterFieldset) — it never pulls Google's events in.
package profile

import (
	"encoding/json"
	"net/http"
	"time"

	config "tls-rest/go/app/constants"
	"tls-rest/go/engine/controllers/db/cache"
	"tls-rest/go/engine/controllers/db/pgdb"
	"tls-rest/go/engine/controllers/functions"
	"tls-rest/go/engine/controllers/httpx"

	"golang.org/x/oauth2"
)

var googleCalendarEndpoint = oauth2.Endpoint{
	AuthURL:  "https://accounts.google.com/o/oauth2/auth",
	TokenURL: "https://oauth2.googleapis.com/token",
}

const googleCalendarScope = "https://www.googleapis.com/auth/calendar.events"

func googleOAuthConfig(r *http.Request) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     config.GoogleID,
		ClientSecret: config.GoogleSecret,
		Endpoint:     googleCalendarEndpoint,
		RedirectURL:  httpx.BaseURL(r) + "/api/profile/google/callback",
		Scopes:       []string{googleCalendarScope},
	}
}

// googleConfigForRefresh is googleOAuthConfig without a RedirectURL — not
// needed for a token refresh, only for the initial AuthCodeURL/Exchange.
// Exported for googlepush.go.
func googleConfigForRefresh() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     config.GoogleID,
		ClientSecret: config.GoogleSecret,
		Endpoint:     googleCalendarEndpoint,
		Scopes:       []string{googleCalendarScope},
	}
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// --- CSRF state cookie (connect -> callback), same pattern as
// auth/providers.go's stateCookie — copied rather than shared, to keep this
// package decoupled from auth.

const connectStateCookie = "gcal_connect_state"

func setConnectStateCookie(w http.ResponseWriter, r *http.Request) string {
	state, _ := functions.GetRandomHash(16)
	http.SetCookie(w, &http.Cookie{
		Name:     connectStateCookie,
		Value:    state,
		Path:     "/",
		Expires:  time.Now().Add(10 * time.Minute),
		HttpOnly: true,
		Secure:   httpx.Scheme(r) == "https",
		SameSite: http.SameSiteLaxMode,
	})
	return state
}

func checkConnectStateCookie(w http.ResponseWriter, r *http.Request) bool {
	defer http.SetCookie(w, &http.Cookie{Name: connectStateCookie, Value: "", Path: "/", MaxAge: -1})
	c, err := r.Cookie(connectStateCookie)
	if err != nil || c.Value == "" {
		return false
	}
	got := r.FormValue("state")
	return got != "" && got == c.Value
}

func requireProfileSession(w http.ResponseWriter, r *http.Request) *cache.Session {
	s := cache.SessionFromContext(r.Context())
	if s == nil || s.UserID <= 0 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return nil
	}
	return s
}

// handleGoogleConnect redirects to Google's consent screen. prompt=consent
// forces it even on repeat connects, so a refresh token always comes back —
// Google omits it on a callback that re-uses a still-valid prior consent.
func handleGoogleConnect(w http.ResponseWriter, r *http.Request) {
	s := requireProfileSession(w, r)
	if s == nil {
		return
	}
	if config.GoogleID == "" || config.GoogleSecret == "" {
		http.Error(w, "Google OAuth is not configured", http.StatusServiceUnavailable)
		return
	}
	state := setConnectStateCookie(w, r)
	url := googleOAuthConfig(r).AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent"),
	)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// handleGoogleCallback exchanges the code for tokens and stores them against
// the signed-in user — googlepush.go reads them per-event from then on.
func handleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	s := requireProfileSession(w, r)
	if s == nil {
		return
	}
	if !checkConnectStateCookie(w, r) {
		http.Redirect(w, r, "/profile?error=state", http.StatusTemporaryRedirect)
		return
	}
	code := r.FormValue("code")
	if code == "" {
		http.Redirect(w, r, "/profile?error=oauth", http.StatusTemporaryRedirect)
		return
	}

	tok, err := googleOAuthConfig(r).Exchange(r.Context(), code)
	if err != nil || tok.RefreshToken == "" {
		// No refresh token means Google skipped the consent screen (shouldn't
		// happen with prompt=consent, but fail loudly rather than silently
		// storing a token that can't be renewed once it expires).
		http.Redirect(w, r, "/profile?error=oauth", http.StatusTemporaryRedirect)
		return
	}

	db, err := pgdb.GetInstanceCtx(r.Context())
	if err != nil {
		http.Redirect(w, r, "/profile?error=oauth", http.StatusTemporaryRedirect)
		return
	}
	_, err = db.Exec(`
		INSERT INTO google_calendar_accounts (user_id, access_token, refresh_token, token_expiry)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id) DO UPDATE SET
			access_token = EXCLUDED.access_token,
			refresh_token = EXCLUDED.refresh_token,
			token_expiry = EXCLUDED.token_expiry
	`, s.UserID, tok.AccessToken, tok.RefreshToken, tok.Expiry)
	if err != nil {
		http.Redirect(w, r, "/profile?error=oauth", http.StatusTemporaryRedirect)
		return
	}

	http.Redirect(w, r, "/profile?connected=1", http.StatusTemporaryRedirect)
}

// handleGoogleDisconnect removes the stored tokens; events already pushed to
// Google are left there (not deleted) and simply stop being updated.
func handleGoogleDisconnect(w http.ResponseWriter, r *http.Request) {
	s := requireProfileSession(w, r)
	if s == nil {
		return
	}
	db, err := pgdb.GetInstanceCtx(r.Context())
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	_, _ = db.Exec(`DELETE FROM google_calendar_accounts WHERE user_id = $1`, s.UserID)
	w.WriteHeader(http.StatusNoContent)
}

// handleGoogleStatus reports whether the signed-in user has a connected account.
func handleGoogleStatus(w http.ResponseWriter, r *http.Request) {
	s := requireProfileSession(w, r)
	if s == nil {
		return
	}
	db, err := pgdb.GetInstanceCtx(r.Context())
	connected := false
	if err == nil {
		row, _ := db.GetOne(`SELECT 1 AS x FROM google_calendar_accounts WHERE user_id = $1`, s.UserID)
		connected = row != nil
	}
	writeJSON(w, map[string]interface{}{"connected": connected})
}
