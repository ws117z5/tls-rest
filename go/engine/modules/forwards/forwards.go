// Package forwards records that a user forwarded a target record, one row
// per forward (no dedup — forwarding the same post to five chats is five
// rows). Written over a small REST API (POST /api/forwards/{module}/{row});
// modules that want a count expose it themselves via a virtual field's SQL
// subquery (see posts.go's forwards_count) rather than calling back into
// this package.
package forwards

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

// Forwards module, backs the forward-count widget on posts.
var Module = &module.ModuleAbstract[interface{}]{
	ID:      "forwards",
	Name:    "Forwards",
	Icon:    "forward",
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
		{Path: "/api/forwards/{module}/{row}", Methods: []string{"POST"}, Handler: handleRecord, Absolute: true},
	},
}

func init() {
	app.RegisterModule(Module, "forwards")
}

func parseTarget(r *http.Request) (modID string, rowID int, ok bool) {
	vars := mux.Vars(r)
	modID = vars["module"]
	rowID, err := strconv.Atoi(vars["row"])
	return modID, rowID, modID != "" && err == nil && rowID >= 0
}

// handleRecord logs one forward of (module, row) by the signed-in caller.
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

	if _, err := db.InsertRow("forwards", map[string]interface{}{
		"module_id": modID,
		"row_id":    rowID,
		"user_id":   s.UserID,
	}); err != nil {
		http.Error(w, "insert failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}
