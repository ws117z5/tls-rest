// Package likes provides a reusable like/dislike reaction that can be attached
// to the records of any module. Reactions live in one polymorphic table keyed
// by (module_id, row_id, user_id) — one reaction per user per target, value
// +1 for a like or -1 for a dislike; reacting the same way again removes it,
// reacting the other way switches it. Read and written over a small REST API
// (GET/POST /api/likes/{module}/{row}); the frontend renders it with a
// dedicated widget embedded by whichever module opts in (posts, comments,
// users), not the generic table widget. The standalone "likes" CRUD module is
// for admin cleanup, same shape as engine/modules/comments.
package likes

import (
	"encoding/json"
	"net/http"
	"strconv"

	"tls-rest/go/app"
	"tls-rest/go/engine/controllers/db/cache"
	"tls-rest/go/engine/controllers/db/pgdb"
	"tls-rest/go/engine/controllers/field"
	"tls-rest/go/engine/controllers/functions"
	"tls-rest/go/engine/controllers/module"

	"github.com/gorilla/mux"
)

// likesFilters declares GET /likes's list filters: module (by module_id) and
// user (by name, via subquery).
func likesFilters() *field.ListFilters {
	return field.NewFieldset(
		field.NewFilter("module", field.TYPE_STRING).WithLabel("Module").WithSQL("module_id").Contains(),
		field.NewFilter("user", field.TYPE_STRING).WithLabel("User").Contains().
			WithSQLWhere("user_id IN (SELECT id FROM users WHERE user_name ILIKE %s)"),
	)
}

// Likes module, backs the like/dislike widget on posts, comments, and users.
var Module = &module.ModuleAbstract[interface{}]{
	ID:      "likes",
	Name:    "Likes",
	Icon:    "like",
	Submenu: "engine",
	Fields: []field.Field{
		field.NewField("module_id", field.TYPE_STRING, true).WithLabel("Module"),
		field.NewField("row_id", field.TYPE_INT, true).WithLabel("Row"),
		field.NewField("user_id", field.TYPE_INT, true).WithLabel("User"),
		field.NewField("value", field.TYPE_INT, true).WithLabel("Value"),
	},
	Filters:              likesFilters(),
	DefaultPermission:    module.PERMISSION_DENY,
	DefaultPermissionSet: true,
	Rights:               make(map[int]int),
	CustomRoutes: []module.CustomRoute{
		{Path: "/api/likes/{module}/{row}", Methods: []string{"GET"}, Handler: handleGet, Absolute: true},
		{Path: "/api/likes/{module}/{row}", Methods: []string{"POST"}, Handler: handleReact, Absolute: true},
	},
}

func init() {
	app.RegisterModule(Module, "likes")
}

// summary is what the client needs to render the widget: aggregate counts
// plus the signed-in user's own reaction (0 when anonymous or unreacted).
type summary struct {
	Likes    int `json:"likes"`
	Dislikes int `json:"dislikes"`
	Mine     int `json:"mine"`
}

func loadSummary(db *pgdb.Db, modID string, rowID int, userID int) (summary, error) {
	row, err := db.GetOne(`
		SELECT
			COALESCE(SUM(CASE WHEN value = 1 THEN 1 ELSE 0 END), 0)  AS likes,
			COALESCE(SUM(CASE WHEN value = -1 THEN 1 ELSE 0 END), 0) AS dislikes
		FROM likes WHERE module_id = $1 AND row_id = $2`, modID, rowID)
	if err != nil {
		return summary{}, err
	}
	s := summary{
		Likes:    functions.Int(row["likes"]),
		Dislikes: functions.Int(row["dislikes"]),
	}
	if userID > 0 {
		mine, err := db.GetOne(
			`SELECT value FROM likes WHERE module_id = $1 AND row_id = $2 AND user_id = $3`,
			modID, rowID, userID,
		)
		if err == nil && mine != nil {
			s.Mine = functions.Int(mine["value"])
		}
	}
	return s, nil
}

func parseTarget(r *http.Request) (modID string, rowID int, ok bool) {
	vars := mux.Vars(r)
	modID = vars["module"]
	rowID, err := strconv.Atoi(vars["row"])
	return modID, rowID, modID != "" && err == nil && rowID >= 0
}

// handleGet returns the current reaction counts for (module, row), plus the
// caller's own reaction if signed in. Works for anonymous callers.
func handleGet(w http.ResponseWriter, r *http.Request) {
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

	userID := 0
	if s := cache.SessionFromContext(r.Context()); s != nil {
		userID = s.UserID
	}

	sum, err := loadSummary(db, modID, rowID, userID)
	if err != nil {
		http.Error(w, "query failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, sum)
}

// handleReact sets, switches, or clears (on repeat) the signed-in user's
// reaction to (module, row), then returns the refreshed summary.
func handleReact(w http.ResponseWriter, r *http.Request) {
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

	var in struct {
		Value int `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || (in.Value != 1 && in.Value != -1) {
		http.Error(w, "invalid value", http.StatusBadRequest)
		return
	}

	db, err := pgdb.GetInstanceCtx(r.Context())
	if err != nil {
		http.Error(w, "db unavailable", http.StatusInternalServerError)
		return
	}

	existing, err := db.GetOne(
		`SELECT id, value FROM likes WHERE module_id = $1 AND row_id = $2 AND user_id = $3`,
		modID, rowID, s.UserID,
	)
	if err != nil {
		http.Error(w, "query failed", http.StatusInternalServerError)
		return
	}

	switch {
	case existing == nil:
		if _, err := db.InsertRow("likes", map[string]interface{}{
			"module_id": modID,
			"row_id":    rowID,
			"user_id":   s.UserID,
			"value":     in.Value,
		}); err != nil {
			http.Error(w, "insert failed", http.StatusInternalServerError)
			return
		}
	case functions.Int(existing["value"]) == in.Value:
		// Reacting the same way again is a toggle-off.
		if _, err := db.DeleteRow("likes", "id", existing["id"]); err != nil {
			http.Error(w, "delete failed", http.StatusInternalServerError)
			return
		}
	default:
		if _, err := db.UpdateRow("likes", map[string]interface{}{"value": in.Value}, "id", existing["id"]); err != nil {
			http.Error(w, "update failed", http.StatusInternalServerError)
			return
		}
	}

	sum, err := loadSummary(db, modID, rowID, s.UserID)
	if err != nil {
		http.Error(w, "query failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, sum)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
