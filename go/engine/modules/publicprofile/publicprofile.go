// Package publicprofile exposes a minimal, privacy-safe public view of a
// user's account to any other signed-in user: username, avatar and join
// date. It deliberately never returns first name, last name or email — those
// stay behind the admin-only, rights-gated "users" module (and the separate
// engine/pages/profile, which is the signed-in user's own editable profile).
// Read-only; there is no admin CRUD module here, just the one endpoint.
package publicprofile

import (
	"encoding/json"
	"net/http"
	"strconv"

	"tls-rest/go/engine/controllers/db/cache"
	"tls-rest/go/engine/controllers/db/pgdb"
	"tls-rest/go/engine/controllers/functions"
	"tls-rest/go/engine/controllers/module"

	"github.com/gorilla/mux"
)

// Init registers the public profile REST API. Call from the registry.
func init() {
	module.RegisterEndpointPrefix("/api/users")
	module.AddRouteRegistrar(func(r *mux.Router) {
		r.HandleFunc("/api/users/{id}/public", handleGet).Methods("GET")
	})
}

type publicProfile struct {
	ID       int64       `json:"id"`
	UserName string      `json:"userName"`
	Image    string      `json:"image"`
	Created  interface{} `json:"created"`
	// Self is true when the caller is viewing their own profile — the
	// frontend uses it to hide the "send message" thread on your own page.
	Self bool `json:"self"`
}

func handleGet(w http.ResponseWriter, r *http.Request) {
	s := cache.SessionFromContext(r.Context())
	if s == nil || s.UserID <= 0 {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	id, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}

	db, err := pgdb.GetInstanceCtx(r.Context())
	if err != nil {
		http.Error(w, "db unavailable", http.StatusInternalServerError)
		return
	}

	row, err := db.GetOne(`SELECT id, user_name, image, created FROM users WHERE id = $1`, id)
	if err != nil {
		http.Error(w, "query failed", http.StatusInternalServerError)
		return
	}
	if row == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	out := publicProfile{
		ID:       id,
		UserName: functions.Coerce[string](row["user_name"]),
		Image:    functions.ImageFieldURL(row["image"]),
		Created:  row["created"],
		Self:     s.UserID == int(id),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}
