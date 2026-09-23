// Package login backs the email/password auth endpoints: /api/login,
// /api/logout, /api/register. OAuth (/users/Auth/{provider}) is handled
// separately by lib/auth.
package login

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"tls-rest/go/app"
	"tls-rest/go/engine/controllers/auth"
	"tls-rest/go/engine/controllers/db/pgdb"
	"tls-rest/go/engine/controllers/functions"
	"tls-rest/go/engine/controllers/module"

	"golang.org/x/crypto/bcrypt"
)

type credentials struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	UserName  string `json:"user_name"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	External  bool   `json:"external"` // true = mobile client, response includes a bearer token
}

// writeAuthResult returns the standard {ok,user} body and, for external clients,
// additionally issues and includes a bearer token. It writes the HTTP response
// (including any error) itself.
func writeAuthResult(ctx context.Context, w http.ResponseWriter, id int, username string, external bool) {
	resp := map[string]interface{}{
		"ok":   true,
		"user": map[string]interface{}{"id": id, "user_name": username},
	}
	if external {
		token, expire, err := auth.IssueToken(ctx, id, username)
		if err != nil {
			functions.JSONError(w, http.StatusInternalServerError, "could not issue token")
			return
		}
		resp["token"] = token
		resp["token_type"] = "Bearer"
		resp["expires"] = expire.UTC().Format(time.RFC3339)
	}
	functions.WriteJSON(w, http.StatusOK, resp)
}

// Login handles POST /api/login {email, password}. On success it establishes an
// authenticated session and returns the basic user info.
func Login(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		functions.JSONError(w, http.StatusBadRequest, "invalid request")
		return
	}
	c.Email = strings.TrimSpace(strings.ToLower(c.Email))
	if c.Email == "" || c.Password == "" {
		functions.JSONError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	db, err := pgdb.GetInstanceCtx(r.Context())
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "database unavailable")
		return
	}

	row, err := db.GetOne(
		`SELECT id, user_name, password_hash FROM users WHERE lower(email) = $1 LIMIT 1`, c.Email,
	)
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if row == nil {
		functions.JSONError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	hash := functions.Coerce[string](row["password_hash"])
	if hash == "" || bcrypt.CompareHashAndPassword([]byte(hash), []byte(c.Password)) != nil {
		functions.JSONError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	id := functions.Coerce[int](row["id"])
	username := functions.Coerce[string](row["user_name"])
	auth.Login(w, r, id, username)

	writeAuthResult(r.Context(), w, id, username, c.External)
}

// Logout handles POST /api/logout, dropping the session back to anonymous.
func Logout(w http.ResponseWriter, r *http.Request) {
	auth.Logout(w, r)
	functions.WriteJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

// oauthCredentials is the body of POST /api/auth/oauth. A non-web client first
// authenticates with the provider itself (native SDK / PKCE) to obtain a provider
// access token, then posts it here to exchange it for our own bearer token.
type oauthCredentials struct {
	Provider    string `json:"provider"`     // "google" | "github" | "facebook" | "vk"
	AccessToken string `json:"access_token"` // provider token from the device
	Email       string `json:"email"`        // optional; used when the provider returns none
	External    bool   `json:"external"`     // true = mobile client, response includes a bearer token
}

// OAuth handles POST /api/auth/oauth {provider, access_token, email?,
// external?}: verifies the provider token, finds/creates the local user,
// establishes a session, and (for external clients) returns a bearer token.
func OAuth(w http.ResponseWriter, r *http.Request) {
	var c oauthCredentials
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		functions.JSONError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if strings.TrimSpace(c.Provider) == "" || strings.TrimSpace(c.AccessToken) == "" {
		functions.JSONError(w, http.StatusBadRequest, "provider and access_token are required")
		return
	}

	id, username, err := auth.ProviderLoginWithToken(r.Context(), r, c.Provider, c.AccessToken, c.Email)
	if err != nil {
		functions.JSONError(w, http.StatusUnauthorized, "oauth authentication failed")
		return
	}

	auth.Login(w, r, id, username)
	writeAuthResult(r.Context(), w, id, username, c.External)
}

// Register handles POST /api/register {email, password, user_name?, first_name?,
// last_name?}. Creates a password account and logs the new user in.
func Register(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		functions.JSONError(w, http.StatusBadRequest, "invalid request")
		return
	}
	c.Email = strings.TrimSpace(strings.ToLower(c.Email))
	if c.Email == "" || len(c.Password) < 6 {
		functions.JSONError(w, http.StatusBadRequest, "email and a password (min 6 chars) are required")
		return
	}

	db, err := pgdb.GetInstanceCtx(r.Context())
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "database unavailable")
		return
	}

	if row, e := db.GetOne(`SELECT id FROM users WHERE lower(email) = $1 LIMIT 1`, c.Email); e == nil && row != nil {
		functions.JSONError(w, http.StatusConflict, "an account with this email already exists")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(c.Password), bcrypt.DefaultCost)
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "could not hash password")
		return
	}

	local := c.Email
	if i := strings.IndexByte(local, '@'); i > 0 {
		local = local[:i]
	}
	firstName := c.FirstName
	if firstName == "" {
		firstName = local
	}
	userName := c.UserName
	if userName == "" {
		userName = local
	}

	id, err := db.InsertRow("users", map[string]interface{}{
		"user_name":     userName,
		"first_name":    firstName,
		"last_name":     c.LastName,
		"email":         c.Email,
		"password_hash": string(hash),
	})
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	auth.Login(w, r, int(id), userName)

	writeAuthResult(r.Context(), w, int(id), userName, c.External)
}

// Page registers the login/auth HTTP endpoints.
var Page = &module.PageAbstract{
	ID:   "login",
	Name: "Login",
	Icon: "login",
	Routes: []module.PageRoute{
		{Path: "/api/login", Methods: []string{"POST"}, Handler: Login},
		{Path: "/api/auth/oauth", Methods: []string{"POST"}, Handler: OAuth},
		{Path: "/api/logout", Methods: []string{"POST"}, Handler: Logout},
		{Path: "/api/register", Methods: []string{"POST"}, Handler: Register},
	},
}

func init() {
	app.RegisterPage(Page)
}
