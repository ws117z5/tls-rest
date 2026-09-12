// Package deletion backs the public "request account deletion" page. Requests
// are stored in the deletion_requests table (read and actioned by admins through
// the CRUD module here); nothing is deleted automatically. A dedicated page and
// endpoint exist because identity providers (e.g. Facebook) require a public,
// login-optional data-deletion request URL.
//
//	POST /api/deletion-request  {email, reason, note}  -> {ok:true}
package deletion

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"tls-rest/go/engine/controllers/db/cache"
	"tls-rest/go/engine/controllers/db/pgdb"
	"tls-rest/go/engine/controllers/field"
	"tls-rest/go/engine/controllers/module"

	"github.com/gorilla/mux"
)

// Module is the admin-only CRUD view over stored deletion requests.
var Module = &module.ModuleAbstract[interface{}]{
	ID:      "deletion_requests",
	Name:    "Deletion Requests",
	Icon:    "user-delete-request",
	Submenu: "engine",
	Fields: []field.Field{
		field.NewField("email", field.TYPE_STRING, true).WithLabel("Email"),
		field.NewField("user_id", field.TYPE_INT, false).WithLabel("User ID"),
		field.NewField("reason", field.TYPE_STRING, false).WithLabel("Reason"),
		field.NewField("note", field.TYPE_TEXT, false).WithLabel("Note"),
		field.NewField("handled", field.TYPE_CHECKBOX, false).WithLabel("Handled").WithDefault(false),
	},
	DefaultPermission:    module.PERMISSION_DENY,
	DefaultPermissionSet: true,
	Rights:               make(map[int]int),
}

func Init() {
	Module.Initialize("deletion_requests")

	module.RegisterEndpointPrefix("/api/deletion-request")
	module.AddRouteRegistrar(func(r *mux.Router) {
		r.HandleFunc("/api/deletion-request", handleSubmit).Methods("POST")
	})
}

var emailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

func clip(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) > max {
		return s[:max]
	}
	return s
}

// handleSubmit validates and stores one deletion request. Public — a signed-in
// session is used to record the account id when present, but is not required
// (a user who has lost provider access must still be able to ask).
func handleSubmit(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email   string `json:"email"`
		Reason  string `json:"reason"`
		Note    string `json:"note"`
		Website string `json:"website"` // honeypot
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(in.Website) != "" {
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
		return
	}

	email := clip(in.Email, 200)
	if !emailRe.MatchString(email) {
		http.Error(w, "a valid email is required", http.StatusBadRequest)
		return
	}

	row := map[string]interface{}{
		"email":   email,
		"reason":  clip(in.Reason, 200),
		"note":    clip(in.Note, 5000),
		"handled": false,
	}
	if s := cache.SessionFromContext(r.Context()); s != nil && s.UserID > 0 {
		row["user_id"] = s.UserID
	}

	db, err := pgdb.GetInstance()
	if err != nil {
		http.Error(w, "unavailable", http.StatusInternalServerError)
		return
	}
	if _, err := db.InsertRow("deletion_requests", row); err != nil {
		http.Error(w, "could not save request", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
