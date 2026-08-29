// Package login backs the email/password authentication endpoints for the login
// Page: /api/login, /api/logout, /api/register. OAuth sign-in is handled
// separately by lib/auth (the /users/Auth/{provider} flow). All three establish
// or clear a session through auth.Login / auth.Logout — the single place that
// sets UserID on a session.
package login

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

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
	// External marks a non-web (mobile) client. When true the response also
	// carries a bearer token the client sends as `Authorization: Bearer <token>`
	// (web clients ignore it and use the cookie session as before).
	External bool `json:"external"`
}

// writeAuthResult returns the standard {ok,user} body and, for external clients,
// additionally issues and includes a bearer token. It writes the HTTP response
// (including any error) itself.
func writeAuthResult(w http.ResponseWriter, id int, username string, external bool) {
	resp := map[string]interface{}{
		"ok":   true,
		"user": map[string]interface{}{"id": id, "user_name": username},
	}
	if external {
		token, expire, err := auth.IssueToken(id, username)
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

	db, err := pgdb.GetInstance()
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
	// Uniform message so we don't reveal whether the email exists.
	if row == nil {
		functions.JSONError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	hash := pgdb.Coerce[string](row["password_hash"])
	if hash == "" || bcrypt.CompareHashAndPassword([]byte(hash), []byte(c.Password)) != nil {
		functions.JSONError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	id := pgdb.Coerce[int](row["id"])
	username := pgdb.Coerce[string](row["user_name"])
	auth.Login(w, r, id, username)

	writeAuthResult(w, id, username, c.External)
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
	// Same flag as password login. External clients (mobile) get a bearer token;
	// a web caller can omit it and rely on the cookie the flow also sets.
	External bool `json:"external"`
}

// OAuth handles POST /api/auth/oauth {provider, access_token, email?, external?}.
// It verifies the provider token (reusing the same provider registry as the web
// /users/Auth/{provider} callback), finds/creates the local user, establishes a
// session, and — for external clients — returns a bearer token.
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
		// Uniform message; the detail is in the server logs.
		functions.JSONError(w, http.StatusUnauthorized, "oauth authentication failed")
		return
	}

	auth.Login(w, r, id, username)
	writeAuthResult(w, id, username, c.External)
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

	db, err := pgdb.GetInstance()
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

	// user_name / first_name are NOT NULL in the schema; default them from the
	// email local-part when not supplied.
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

	writeAuthResult(w, int(id), userName, c.External)
}

// Page self-registers the auth endpoints through the shared route-registrar seam
// so route.go no longer hardcodes them. Login is a custom (non-fieldset) page:
// it owns its handlers rather than a fieldset. OAuth (/users/Auth/...) is handled
// separately by lib/auth.
var Page = &module.PageAbstract{
	ID:   "login",
	Name: "Login",
	Routes: []module.PageRoute{
		{Path: "/api/login", Methods: []string{"POST"}, Handler: Login},
		{Path: "/api/auth/oauth", Methods: []string{"POST"}, Handler: OAuth},
		{Path: "/api/logout", Methods: []string{"POST"}, Handler: Logout},
		{Path: "/api/register", Methods: []string{"POST"}, Handler: Register},
	},
}

func Init() {
	Page.Initialize()
}
