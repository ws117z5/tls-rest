// Package userconfig serves the current user's resolved config to the client.
// It is a plain API endpoint (GET /api/config), NOT a menu page — registered
// directly in the router so it never appears in the navigation.
package userconfig

import (
	"net/http"

	"tls-rest/go/engine/controllers/config"
	"tls-rest/go/engine/controllers/db/cache"
	"tls-rest/go/engine/controllers/functions"
)

// Get handles GET /api/config, returning the effective config for the caller
// (defaults < global < group < user), taken from the session cache.
func Get(w http.ResponseWriter, r *http.Request) {
	s := cache.SessionFromContext(r.Context())
	if s != nil && s.Config != nil {
		functions.WriteJSON(w, http.StatusOK, s.Config)
		return
	}
	uid := 0
	if s != nil {
		uid = s.UserID
	}
	functions.WriteJSON(w, http.StatusOK, config.Resolve(uid))
}
