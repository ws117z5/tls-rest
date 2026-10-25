// Package friends provides a simple friend-request relationship between two
// users: one row per pair, direction-agnostic once accepted. A request starts
// "pending" (requester_id -> recipient_id) and becomes "accepted" once the
// recipient accepts; either side can remove the relationship at any point
// (which also covers cancelling an outgoing request or declining an incoming
// one — there is nothing left to distinguish once the row is gone).
//
//	GET  /api/friends/list           -> {friends: [{id, userName, image}]} (accepted only)
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
	"context"
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

// Friends module, backs FriendButton and the friend-request flow.
var Module = &module.ModuleAbstract[interface{}]{
	ID:      "friend_requests",
	Name:    "Friend Requests",
	Icon:    "user-groups",
	Submenu: "users",
	Fields: []field.Field{
		field.NewField("requester_id", field.TYPE_INT, true).WithLabel("Requester"),
		field.NewField("recipient_id", field.TYPE_INT, true).WithLabel("Recipient"),
		field.NewField("status", field.TYPE_STRING, true).WithLabel("Status"),
	},
	DefaultPermission:    module.PERMISSION_DENY,
	DefaultPermissionSet: true,
	Rights:               make(map[int]int),
	CustomRoutes: []module.CustomRoute{
		{Path: "/api/friends/list", Methods: []string{"GET"}, Handler: handleList, Absolute: true},
		{Path: "/api/friends/status/{id}", Methods: []string{"GET"}, Handler: handleStatus, Absolute: true},
		{Path: "/api/friends/request/{id}", Methods: []string{"POST"}, Handler: handleRequest, Absolute: true},
		{Path: "/api/friends/accept/{id}", Methods: []string{"POST"}, Handler: handleAccept, Absolute: true},
		{Path: "/api/friends/remove/{id}", Methods: []string{"POST"}, Handler: handleRemove, Absolute: true},
	},
}

func init() {
	app.RegisterModule(Module, "friends")
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

// AreFriends reports whether the two users have an accepted friend
// relationship. Exported for other modules that need to gate something on
// friendship without duplicating this query — see p2prendezvous.
func AreFriends(ctx context.Context, userA, userB int) (bool, error) {
	db, err := pgdb.GetInstanceCtx(ctx)
	if err != nil {
		return false, err
	}
	row, err := db.GetOne(`
		SELECT 1 FROM friends
		WHERE status = 'accepted' AND ((requester_id = $1 AND recipient_id = $2) OR (requester_id = $2 AND recipient_id = $1))`,
		userA, userB)
	if err != nil {
		return false, err
	}
	return row != nil, nil
}

// ListFriendIDs returns the user ids of every accepted friend of userID.
// Exported for modules that only need the ids (e.g. p2prendezvous's presence
// snapshot), without the join/HTTP overhead of the full handleList response.
func ListFriendIDs(ctx context.Context, userID int) ([]int, error) {
	db, err := pgdb.GetInstanceCtx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := db.GetAll(`
		SELECT CASE WHEN requester_id = $1 THEN recipient_id ELSE requester_id END AS other_id
		FROM friends
		WHERE status = 'accepted' AND (requester_id = $1 OR recipient_id = $1)`, userID)
	if err != nil {
		return nil, err
	}
	out := make([]int, 0, len(rows))
	for _, row := range rows {
		out = append(out, functions.Int(row["other_id"]))
	}
	return out, nil
}

// handleList returns every accepted friend of the caller.
func handleList(w http.ResponseWriter, r *http.Request) {
	s := requireSession(w, r)
	if s == nil {
		return
	}

	db, err := pgdb.GetInstanceCtx(r.Context())
	if err != nil {
		http.Error(w, "db unavailable", http.StatusInternalServerError)
		return
	}

	rows, err := db.GetAll(`
		SELECT u.id, u.user_name, u.image
		FROM friends f
		JOIN users u ON u.id = CASE WHEN f.requester_id = $1 THEN f.recipient_id ELSE f.requester_id END
		WHERE f.status = 'accepted' AND (f.requester_id = $1 OR f.recipient_id = $1)
		ORDER BY u.user_name`, s.UserID)
	if err != nil {
		http.Error(w, "query failed", http.StatusInternalServerError)
		return
	}

	type friendRow struct {
		ID       int64  `json:"id"`
		UserName string `json:"userName"`
		Image    string `json:"image"`
	}
	out := make([]friendRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, friendRow{
			ID:       functions.Coerce[int64](row["id"]),
			UserName: functions.Coerce[string](row["user_name"]),
			Image:    functions.ImageFieldURL(row["image"]),
		})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"friends": out})
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

	db, err := pgdb.GetInstanceCtx(r.Context())
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

	db, err := pgdb.GetInstanceCtx(r.Context())
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

	db, err := pgdb.GetInstanceCtx(r.Context())
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

	db, err := pgdb.GetInstanceCtx(r.Context())
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
