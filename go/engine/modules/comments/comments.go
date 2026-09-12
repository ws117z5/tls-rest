// Package comments provides a reusable discussion thread that can be attached to
// the records of any module. Comments live in one polymorphic table keyed by
// (module_id, row_id):
//
//   - a top-level comment on a post has module_id="posts", row_id=<post id>
//   - a reply to a comment has  module_id="comments", row_id=<parent comment id>
//
// so a thread is an arbitrarily deep tree. The thread is read and written over a
// small REST API (GET/POST /api/comments/{module}/{row}); the frontend renders
// it with a dedicated component, not the generic table widget. The standalone
// "comments" CRUD module is for admin cleanup.
package comments

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"tls-rest/go/engine/controllers/db/cache"
	"tls-rest/go/engine/controllers/db/pgdb"
	"tls-rest/go/engine/controllers/field"
	"tls-rest/go/engine/controllers/functions"
	"tls-rest/go/engine/controllers/module"

	"github.com/gorilla/mux"
)

// selfModule is the module_id a comment carries when it is a reply to another
// comment (its row_id is then the parent comment's id).
const selfModule = "comments"

// Module is the admin-only CRUD view over the raw comments table. The table
// itself is created from this fieldset by the engine (plus init/sql for the
// (module_id, row_id) index).
var Module = &module.ModuleAbstract[interface{}]{
	ID:      "comments",
	Name:    "Comments",
	Icon:    "comments",
	Submenu: "engine",
	Fields: []field.Field{
		field.NewField("module_id", field.TYPE_STRING, true).WithLabel("Module"),
		field.NewField("row_id", field.TYPE_INT, true).WithLabel("Row"),
		field.NewField("body", field.TYPE_TEXT, true).WithLabel("Comment"),
	},
	DefaultPermission:    module.PERMISSION_DENY,
	DefaultPermissionSet: true,
	Rights:               make(map[int]int),
}

// Init registers the standalone comments module and the thread REST API. Call
// from the registry.
func Init() {
	Module.Initialize("comments")

	module.RegisterEndpointPrefix("/api/comments")
	module.AddRouteRegistrar(func(r *mux.Router) {
		r.HandleFunc("/api/comments/{module}/{row}", handleList).Methods("GET")
		r.HandleFunc("/api/comments/{module}/{row}", handleCreate).Methods("POST")
	})
}

// node is one comment plus its nested replies, as sent to the client.
type node struct {
	ID      int         `json:"id"`
	Author  string      `json:"author"`
	Body    string      `json:"body"`
	Created interface{} `json:"created"`
	Replies []*node     `json:"replies"`
}

// handleList returns the whole comment tree rooted at (module, row).
func handleList(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	modID := vars["module"]
	rowID, err := strconv.Atoi(vars["row"])
	if modID == "" || err != nil || rowID < 0 {
		http.Error(w, "bad target", http.StatusBadRequest)
		return
	}

	db, err := pgdb.GetInstance()
	if err != nil {
		http.Error(w, "db unavailable", http.StatusInternalServerError)
		return
	}

	// One recursive walk: the anchor is every comment directly on (module, row);
	// each step down follows comments whose parent is a comment already in the set.
	rows, err := db.GetAll(`
		WITH RECURSIVE thread AS (
			SELECT c.id, c.module_id, c.row_id, c.body, c.created, c.created_by
			FROM comments c
			WHERE c.module_id = $1 AND c.row_id = $2
		  UNION ALL
			SELECT c.id, c.module_id, c.row_id, c.body, c.created, c.created_by
			FROM comments c
			JOIN thread t ON c.module_id = '`+selfModule+`' AND c.row_id = t.id
		)
		SELECT t.id, t.module_id, t.row_id, t.body, t.created,
		       COALESCE(NULLIF(TRIM(CONCAT(u.first_name, ' ', u.last_name)), ''),
		                u.user_name, '') AS author
		FROM thread t
		LEFT JOIN users u ON u.id = t.created_by
		ORDER BY t.created`, modID, rowID)
	if err != nil {
		http.Error(w, "query failed", http.StatusInternalServerError)
		return
	}

	byID := make(map[int]*node, len(rows))
	order := make([]*node, 0, len(rows))
	for _, row := range rows {
		n := &node{
			ID:      functions.Int(row["id"]),
			Author:  functions.Coerce[string](row["author"]),
			Body:    functions.Coerce[string](row["body"]),
			Created: row["created"],
		}
		byID[n.ID] = n
		order = append(order, n)
	}

	roots := make([]*node, 0)
	for i, row := range rows {
		n := order[i]
		parentIsComment, _ := row["module_id"].(string)
		if parentIsComment == selfModule {
			if p := byID[functions.Int(row["row_id"])]; p != nil {
				p.Replies = append(p.Replies, n)
				continue
			}
		}
		roots = append(roots, n)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"comments": roots})
}

// handleCreate appends one comment to (module, row). Requires a signed-in user.
func handleCreate(w http.ResponseWriter, r *http.Request) {
	s := cache.SessionFromContext(r.Context())
	if s == nil || s.UserID <= 0 {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	modID := vars["module"]
	rowID, err := strconv.Atoi(vars["row"])
	if modID == "" || err != nil || rowID < 0 {
		http.Error(w, "bad target", http.StatusBadRequest)
		return
	}

	var in struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	body := strings.TrimSpace(in.Body)
	if body == "" {
		http.Error(w, "empty comment", http.StatusBadRequest)
		return
	}

	db, err := pgdb.GetInstance()
	if err != nil {
		http.Error(w, "db unavailable", http.StatusInternalServerError)
		return
	}
	id, err := db.InsertRow("comments", map[string]interface{}{
		"module_id":  modID,
		"row_id":     rowID,
		"body":       body,
		"created_by": s.UserID,
	})
	if err != nil {
		http.Error(w, "insert failed", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"id": id})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
