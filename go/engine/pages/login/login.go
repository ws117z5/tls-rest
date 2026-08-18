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

	engine "tls-rest/go/engine"
	"tls-rest/go/lib"
	"tls-rest/go/lib/auth"
	pgdb "tls-rest/go/lib/db/pgdb"

	"golang.org/x/crypto/bcrypt"
)

// Page self-registers the auth endpoints through the shared route-registrar seam
// so route.go no longer hardcodes them. Login is a custom (non-fieldset) page:
// it owns its handlers rather than a fieldset. OAuth (/users/Auth/...) is handled
// separately by lib/auth.
var Page = &engine.PageAbstract{
	ID:   "login",
	Name: "Login",
	Routes: []engine.PageRoute{
		{Path: "/api/login", Methods: []string{"POST"}, Handler: Login},
		{Path: "/api/logout", Methods: []string{"POST"}, Handler: Logout},
		{Path: "/api/register", Methods: []string{"POST"}, Handler: Register},
	},
}

func init() {
	Page.Initialize()
}

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
		lib.JSONError(w, http.StatusBadRequest, "invalid request")
		return
	}
	c.Email = strings.TrimSpace(strings.ToLower(c.Email))
	if c.Email == "" || c.Password == "" {
		lib.JSONError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	db, err := pgdb.GetInstance()
	if err != nil {
		lib.JSONError(w, http.StatusInternalServerError, "database unavailable")
		return
	}

	rows, err := db.RQuery(
		`SELECT id, user_name, password_hash FROM users WHERE lower(email) = $1 LIMIT 1`, c.Email,
	)
	if err != nil {
		lib.JSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Uniform message so we don't reveal whether the email exists.
	if len(rows) == 0 {
		lib.JSONError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	hash := pgdb.AsString(rows[0]["password_hash"])
	if hash == "" || bcrypt.CompareHashAndPassword([]byte(hash), []byte(c.Password)) != nil {
		lib.JSONError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	id := int(pgdb.AsInt64(rows[0]["id"]))
	username := pgdb.AsString(rows[0]["user_name"])
	auth.Login(w, r, id, username)

	lib.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"ok":   true,
		"user": map[string]interface{}{"id": id, "user_name": username},
	})
}

// Logout handles POST /api/logout, dropping the session back to anonymous.
func Logout(w http.ResponseWriter, r *http.Request) {
	auth.Logout(w, r)
	lib.WriteJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

// Register handles POST /api/register {email, password, user_name?, first_name?,
// last_name?}. Creates a password account and logs the new user in.
func Register(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		lib.JSONError(w, http.StatusBadRequest, "invalid request")
		return
	}
	c.Email = strings.TrimSpace(strings.ToLower(c.Email))
	if c.Email == "" || len(c.Password) < 6 {
		lib.JSONError(w, http.StatusBadRequest, "email and a password (min 6 chars) are required")
		return
	}

	db, err := pgdb.GetInstance()
	if err != nil {
		lib.JSONError(w, http.StatusInternalServerError, "database unavailable")
		return
	}

	if rows, e := db.RQuery(`SELECT id FROM users WHERE lower(email) = $1 LIMIT 1`, c.Email); e == nil && len(rows) > 0 {
		lib.JSONError(w, http.StatusConflict, "an account with this email already exists")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(c.Password), bcrypt.DefaultCost)
	if err != nil {
		lib.JSONError(w, http.StatusInternalServerError, "could not hash password")
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

	ins, err := db.RQuery(
		`INSERT INTO users (user_name, first_name, last_name, email, password_hash)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		userName, firstName, c.LastName, c.Email, string(hash),
	)
	if err != nil || len(ins) == 0 {
		msg := "could not create account"
		if err != nil {
			msg = err.Error()
		}
		lib.JSONError(w, http.StatusInternalServerError, msg)
		return
	}

	id := int(pgdb.AsInt64(ins[0]["id"]))
	auth.Login(w, r, id, userName)

	lib.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"ok":   true,
		"user": map[string]interface{}{"id": id, "user_name": userName},
	})
}
