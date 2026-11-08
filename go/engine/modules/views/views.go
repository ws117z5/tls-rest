// Package views records that a user has seen a target record, one row per
// (module_id, row_id, user_id) — a view can't be un-viewed, so recording is
// idempotent rather than a toggle like engine/modules/likes. Read and written
// over a small REST API (GET/POST /api/views/{module}/{row}); modules that
// want a count expose it themselves via a virtual field's SQL subquery (see
// posts.go's views_count) rather than calling back into this package.
package views

import (
	"encoding/json"
	"net/http"
	"strconv"

	"tls-rest/go/app"
	"tls-rest/go/engine/controllers/db/cache"
	"tls-rest/go/engine/controllers/db/pgdb"
	"tls-rest/go/engine/controllers/field"
	"tls-rest/go/engine/controllers/module"
	"tls-rest/go/engine/modules/comments"

	"github.com/gorilla/mux"
)

// Views module, backs the view-count widget on posts.
var Module = &module.ModuleAbstract[interface{}]{
	ID:      "views",
	Name:    "Views",
	Icon:    "view",
	Submenu: "engine",
	Fields: []field.Field{
		field.NewField("module_id", field.TYPE_STRING, true).WithLabel("Module"),
		field.NewField("row_id", field.TYPE_INT, true).WithLabel("Row"),
		field.NewField("user_id", field.TYPE_INT, true).WithLabel("User"),
	},
	DefaultPermission:    module.PERMISSION_DENY,
	DefaultPermissionSet: true,
	Rights:               make(map[int]int),
	CustomRoutes: []module.CustomRoute{
		{Path: "/api/views/{module}/{row}", Methods: []string{"GET"}, Handler: handleGet, Absolute: true},
		{Path: "/api/views/{module}/{row}", Methods: []string{"POST"}, Handler: handleRecord, Absolute: true},
	},
}

func init() {
	app.RegisterModule(Module, "views")
}

func parseTarget(r *http.Request) (modID string, rowID int, ok bool) {
	vars := mux.Vars(r)
	modID = vars["module"]
	rowID, err := strconv.Atoi(vars["row"])
	return modID, rowID, modID != "" && err == nil && rowID >= 0
}

// handleGet reports whether the signed-in caller has already viewed (module, row).
func handleGet(w http.ResponseWriter, r *http.Request) {
	s := cache.SessionFromContext(r.Context())
	if s == nil || s.UserID <= 0 {
		writeJSON(w, http.StatusOK, map[string]bool{"viewed": false})
		return
	}

	modID, rowID, ok := parseTarget(r)
	if !ok {
		http.Error(w, "bad target", http.StatusBadRequest)
		return
	}

	db, err := pgdb.GetInstanceCtx(r.Context())
	if err != nil {
		http.Error(w, "db unavailable", http.StatusInternalServerError)
		return
	}

	existing, err := db.GetOne(
		`SELECT id FROM views WHERE module_id = $1 AND row_id = $2 AND user_id = $3`,
		modID, rowID, s.UserID,
	)
	if err != nil {
		http.Error(w, "query failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"viewed": existing != nil})
}

// handleRecord records a view, once, for the signed-in caller. Idempotent —
// a second call for the same (module, row, user) is a no-op, not an error.
func handleRecord(w http.ResponseWriter, r *http.Request) {
	s := cache.SessionFromContext(r.Context())
	if s == nil || s.UserID <= 0 {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	modID, rowID, ok := parseTarget(r)
	if !ok {
		http.Error(w, "bad target", http.StatusBadRequest)
		return
	}
	if !comments.CanViewTarget(r, modID, rowID) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	db, err := pgdb.GetInstanceCtx(r.Context())
	if err != nil {
		http.Error(w, "db unavailable", http.StatusInternalServerError)
		return
	}

	existing, err := db.GetOne(
		`SELECT id FROM views WHERE module_id = $1 AND row_id = $2 AND user_id = $3`,
		modID, rowID, s.UserID,
	)
	if err != nil {
		http.Error(w, "query failed", http.StatusInternalServerError)
		return
	}
	if existing == nil {
		if _, err := db.InsertRow("views", map[string]interface{}{
			"module_id": modID,
			"row_id":    rowID,
			"user_id":   s.UserID,
		}); err != nil {
			http.Error(w, "insert failed", http.StatusInternalServerError)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]bool{"viewed": true})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
