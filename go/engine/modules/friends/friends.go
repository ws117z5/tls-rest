// Package friends provides a simple friend-request relationship between two
// users: one row per pair, direction-agnostic once accepted. A request starts
// "pending" (requester_id -> recipient_id) and becomes "accepted" once the
// recipient accepts; either side can remove the relationship at any point
// (which also covers cancelling an outgoing request or declining an incoming
// one — there is nothing left to distinguish once the row is gone).
//
//	GET  /api/friends/status/{id}   -> {status: "none"|"pending_sent"|"pending_received"|"friends"}
//	POST /api/friends/request/{id}  -> send a friend request to user {id}
//	POST /api/friends/accept/{id}   -> accept an incoming request from user {id}
//	POST /api/friends/remove/{id}   -> remove the relationship with user {id}
//
// The standalone "friend_requests" CRUD module is for admin cleanup, same
// shape as engine/modules/comments. Its ID is deliberately not "friends" to
// leave that name free for a future user-facing page.
package friends

import (
	"encoding/json"
	"net/http"
	"strconv"

	"tls-rest/go/engine/controllers/db/cache"
	"tls-rest/go/engine/controllers/db/pgdb"
	"tls-rest/go/engine/controllers/field"
	"tls-rest/go/engine/controllers/functions"
	"tls-rest/go/engine/controllers/module"

	"github.com/gorilla/mux"
)

// Friends module, backs FriendButton and the friend-request flow.
var Module = &module.ModuleAbstract[interface{}]{
	ID:      "friend_requests",
	Name:    "Friend Requests",
	Icon:    "user-groups",
	Submenu: "engine",
	Fields: []field.Field{
		field.NewField("requester_id", field.TYPE_INT, true).WithLabel("Requester"),
		field.NewField("recipient_id", field.TYPE_INT, true).WithLabel("Recipient"),
		field.NewField("status", field.TYPE_STRING, true).WithLabel("Status"),
	},
	DefaultPermission:    module.PERMISSION_DENY,
	DefaultPermissionSet: true,
	Rights:               make(map[int]int),
}

// Init registers the standalone friend-requests module and the friends REST
// API. Call from the registry.
func Init() {
	Module.Initialize("friends")

	module.RegisterEndpointPrefix("/api/friends")
	module.AddRouteRegistrar(func(r *mux.Router) {
		r.HandleFunc("/api/friends/status/{id}", handleStatus).Methods("GET")
		r.HandleFunc("/api/friends/request/{id}", handleRequest).Methods("POST")
		r.HandleFunc("/api/friends/accept/{id}", handleAccept).Methods("POST")
		r.HandleFunc("/api/friends/remove/{id}", handleRemove).Methods("POST")
	})
}

func requireSession(w http.ResponseWriter, r *http.Request) *cache.Session {
	s := cache.SessionFromContext(r.Context())
	if s == nil || s.UserID <= 0 {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return nil
	}
	return s
}

func targetID(r *http.Request) (int, bool) {
	id, err := strconv.Atoi(mux.Vars(r)["id"])
	return id, err == nil && id > 0
}

// handleStatus reports the caller's relationship with {id}.
func handleStatus(w http.ResponseWriter, r *http.Request) {
	s := requireSession(w, r)
	if s == nil {
		return
	}
	otherID, ok := targetID(r)
	if !ok {
		http.Error(w, "bad user", http.StatusBadRequest)
		return
	}

	db, err := pgdb.GetInstance()
	if err != nil {
		http.Error(w, "db unavailable", http.StatusInternalServerError)
		return
	}

	row, err := db.GetOne(`
		SELECT requester_id, status FROM friends
		WHERE (requester_id = $1 AND recipient_id = $2) OR (requester_id = $2 AND recipient_id = $1)`,
		s.UserID, otherID)
	if err != nil {
		http.Error(w, "query failed", http.StatusInternalServerError)
		return
	}

	status := "none"
	if row != nil {
		if functions.Coerce[string](row["status"]) == "accepted" {
			status = "friends"
		} else if functions.Int(row["requester_id"]) == s.UserID {
			status = "pending_sent"
		} else {
			status = "pending_received"
		}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"status": status})
}

// handleRequest sends a friend request from the caller to {id}.
func handleRequest(w http.ResponseWriter, r *http.Request) {
	s := requireSession(w, r)
	if s == nil {
		return
	}
	otherID, ok := targetID(r)
	if !ok || otherID == s.UserID {
		http.Error(w, "bad user", http.StatusBadRequest)
		return
	}

	db, err := pgdb.GetInstance()
	if err != nil {
		http.Error(w, "db unavailable", http.StatusInternalServerError)
		return
	}

	target, err := db.GetOne(`SELECT id FROM users WHERE id = $1`, otherID)
	if err != nil {
		http.Error(w, "query failed", http.StatusInternalServerError)
		return
	}
	if target == nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	existing, err := db.GetOne(`
		SELECT id FROM friends
		WHERE (requester_id = $1 AND recipient_id = $2) OR (requester_id = $2 AND recipient_id = $1)`,
		s.UserID, otherID)
	if err != nil {
		http.Error(w, "query failed", http.StatusInternalServerError)
		return
	}
	if existing != nil {
		http.Error(w, "relationship already exists", http.StatusConflict)
		return
	}

	if _, err := db.InsertRow("friends", map[string]interface{}{
		"requester_id": s.UserID,
		"recipient_id": otherID,
		"status":       "pending",
		"created_by":   s.UserID,
	}); err != nil {
		http.Error(w, "insert failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "pending_sent"})
}

// handleAccept accepts an incoming pending request from {id}.
func handleAccept(w http.ResponseWriter, r *http.Request) {
	s := requireSession(w, r)
	if s == nil {
		return
	}
	otherID, ok := targetID(r)
	if !ok {
		http.Error(w, "bad user", http.StatusBadRequest)
		return
	}

	db, err := pgdb.GetInstance()
	if err != nil {
		http.Error(w, "db unavailable", http.StatusInternalServerError)
		return
	}

	row, err := db.GetOne(
		`SELECT id FROM friends WHERE requester_id = $1 AND recipient_id = $2 AND status = 'pending'`,
		otherID, s.UserID,
	)
	if err != nil {
		http.Error(w, "query failed", http.StatusInternalServerError)
		return
	}
	if row == nil {
		http.Error(w, "no pending request", http.StatusNotFound)
		return
	}

	if _, err := db.UpdateRow("friends", map[string]interface{}{"status": "accepted"}, "id", row["id"]); err != nil {
		http.Error(w, "update failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "friends"})
}

// handleRemove deletes the relationship with {id} — covers unfriending,
// cancelling an outgoing request, and declining an incoming one alike.
func handleRemove(w http.ResponseWriter, r *http.Request) {
	s := requireSession(w, r)
	if s == nil {
		return
	}
	otherID, ok := targetID(r)
	if !ok {
		http.Error(w, "bad user", http.StatusBadRequest)
		return
	}

	db, err := pgdb.GetInstance()
	if err != nil {
		http.Error(w, "db unavailable", http.StatusInternalServerError)
		return
	}

	if _, err := db.Exec(
		`DELETE FROM friends WHERE (requester_id = $1 AND recipient_id = $2) OR (requester_id = $2 AND recipient_id = $1)`,
		s.UserID, otherID,
	); err != nil {
		http.Error(w, "delete failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "none"})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
