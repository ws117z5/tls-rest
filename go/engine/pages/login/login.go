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

	functions.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"ok":   true,
		"user": map[string]interface{}{"id": id, "user_name": username},
	})
}

// Logout handles POST /api/logout, dropping the session back to anonymous.
func Logout(w http.ResponseWriter, r *http.Request) {
	auth.Logout(w, r)
	functions.WriteJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
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

	functions.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"ok":   true,
		"user": map[string]interface{}{"id": id, "user_name": userName},
	})
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
		{Path: "/api/logout", Methods: []string{"POST"}, Handler: Logout},
		{Path: "/api/register", Methods: []string{"POST"}, Handler: Register},
	},
}

func Init() {
	Page.Initialize()
}
