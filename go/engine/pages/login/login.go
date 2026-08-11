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

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func errMsg(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func asInt(v interface{}) int {
	switch n := v.(type) {
	case int64:
		return int(n)
	case int:
		return n
	case int32:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}

func asString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// Login handles POST /api/login {email, password}. On success it establishes an
// authenticated session and returns the basic user info.
func Login(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		errMsg(w, http.StatusBadRequest, "invalid request")
		return
	}
	c.Email = strings.TrimSpace(strings.ToLower(c.Email))
	if c.Email == "" || c.Password == "" {
		errMsg(w, http.StatusBadRequest, "email and password are required")
		return
	}

	db, err := pgdb.GetInstance()
	if err != nil {
		errMsg(w, http.StatusInternalServerError, "database unavailable")
		return
	}

	rows, err := db.RQuery(
		`SELECT id, user_name, password_hash FROM users WHERE lower(email) = $1 LIMIT 1`, c.Email,
	)
	if err != nil {
		errMsg(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Uniform message so we don't reveal whether the email exists.
	if len(rows) == 0 {
		errMsg(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	hash := asString(rows[0]["password_hash"])
	if hash == "" || bcrypt.CompareHashAndPassword([]byte(hash), []byte(c.Password)) != nil {
		errMsg(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	id := asInt(rows[0]["id"])
	username := asString(rows[0]["user_name"])
	auth.Login(w, r, id, username)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":   true,
		"user": map[string]interface{}{"id": id, "user_name": username},
	})
}

// Logout handles POST /api/logout, dropping the session back to anonymous.
func Logout(w http.ResponseWriter, r *http.Request) {
	auth.Logout(w, r)
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

// Register handles POST /api/register {email, password, user_name?, first_name?,
// last_name?}. Creates a password account and logs the new user in.
func Register(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		errMsg(w, http.StatusBadRequest, "invalid request")
		return
	}
	c.Email = strings.TrimSpace(strings.ToLower(c.Email))
	if c.Email == "" || len(c.Password) < 6 {
		errMsg(w, http.StatusBadRequest, "email and a password (min 6 chars) are required")
		return
	}

	db, err := pgdb.GetInstance()
	if err != nil {
		errMsg(w, http.StatusInternalServerError, "database unavailable")
		return
	}

	if rows, e := db.RQuery(`SELECT id FROM users WHERE lower(email) = $1 LIMIT 1`, c.Email); e == nil && len(rows) > 0 {
		errMsg(w, http.StatusConflict, "an account with this email already exists")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(c.Password), bcrypt.DefaultCost)
	if err != nil {
		errMsg(w, http.StatusInternalServerError, "could not hash password")
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
		errMsg(w, http.StatusInternalServerError, msg)
		return
	}

	id := asInt(ins[0]["id"])
	auth.Login(w, r, id, userName)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":   true,
		"user": map[string]interface{}{"id": id, "user_name": userName},
	})
}
