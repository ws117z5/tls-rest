// Package contact backs the site's contact form. Submissions are stored in the
// contact_messages table (read by admins through the CRUD module here) rather
// than emailed, so the owner's address is never exposed on the site.
//
//	POST /api/contact  {name, email, subject, message}  -> {ok:true}
package contact

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"tls-rest/go/engine/controllers/db/pgdb"
	"tls-rest/go/engine/controllers/field"
	"tls-rest/go/engine/controllers/module"

	"github.com/gorilla/mux"
)

// Module is the admin-only CRUD view over stored contact submissions. The table
// is auto-created from this fieldset by the engine (init/sql pins the types).
var Module = &module.ModuleAbstract[interface{}]{
	ID:      "contact_messages",
	Name:    "Contact Messages",
	Icon:    "messages",
	Submenu: "engine",
	Fields: []field.Field{
		field.NewField("name", field.TYPE_STRING, true).WithLabel("Name"),
		field.NewField("email", field.TYPE_STRING, true).WithLabel("Email"),
		field.NewField("subject", field.TYPE_STRING, false).WithLabel("Subject"),
		field.NewField("message", field.TYPE_TEXT, true).WithLabel("Message"),
		field.NewField("handled", field.TYPE_CHECKBOX, false).WithLabel("Handled").WithDefault(false),
	},
	DefaultPermission:    module.PERMISSION_DENY,
	DefaultPermissionSet: true,
	Rights:               make(map[int]int),
}

func Init() {
	Module.Initialize("contact_messages")

	module.RegisterEndpointPrefix("/api/contact")
	module.AddRouteRegistrar(func(r *mux.Router) {
		r.HandleFunc("/api/contact", handleSubmit).Methods("POST")
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

// handleSubmit validates and stores one contact-form submission. Public — no
// session required. `website` is a honeypot: real users never see or fill it.
func handleSubmit(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name    string `json:"name"`
		Email   string `json:"email"`
		Subject string `json:"subject"`
		Message string `json:"message"`
		Website string `json:"website"` // honeypot
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(in.Website) != "" {
		// Bot filled the hidden field — accept silently, store nothing.
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
		return
	}

	name := clip(in.Name, 120)
	email := clip(in.Email, 200)
	subject := clip(in.Subject, 200)
	message := clip(in.Message, 5000)

	if name == "" || message == "" || !emailRe.MatchString(email) {
		http.Error(w, "name, a valid email, and a message are required", http.StatusBadRequest)
		return
	}

	db, err := pgdb.GetInstance()
	if err != nil {
		http.Error(w, "unavailable", http.StatusInternalServerError)
		return
	}
	if _, err := db.InsertRow("contact_messages", map[string]interface{}{
		"name":    name,
		"email":   email,
		"subject": subject,
		"message": message,
		"handled": false,
	}); err != nil {
		http.Error(w, "could not save message", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
