// Package chats is the persistent replacement for this session's earlier
// friends module and p2prendezvous's code-based rooms: a "chat" (1:1 or
// group) is the only relationship that gates who can message whom. Message
// CONTENT never lives here or anywhere server-side — this package only
// tracks membership/name/avatar/admin; go/engine/modules/p2prendezvous reads
// it to decide who a client may UDP-rendezvous with.
//
//	GET    /api/chats                       -> the caller's chats (any status)
//	POST   /api/chats/direct/{userId}       -> create-or-get a 1:1 chat
//	POST   /api/chats/{id}/accept           -> accept caller's own pending membership
//	POST   /api/chats/{id}/decline          -> decline (remove) caller's own pending membership
//	POST   /api/chats/group                 -> {name, memberIds[]}: create a group chat
//	POST   /api/chats/{id}/members          -> admin-only: add one more member
//	DELETE /api/chats/{id}/members/{userId} -> admin removes anyone; anyone removes themselves (leave)
//	PUT    /api/chats/{id}                  -> admin-only: rename / re-avatar
//	GET    /api/users/search?q=             -> see search.go
package chats

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"

	"tls-rest/go/engine/controllers/db/cache"
	"tls-rest/go/engine/controllers/db/pgdb"
	"tls-rest/go/engine/controllers/functions"
	"tls-rest/go/engine/controllers/module"
)

func requireSession(w http.ResponseWriter, r *http.Request) *cache.Session {
	s := cache.SessionFromContext(r.Context())
	if s == nil || s.UserID <= 0 {
		functions.JSONError(w, http.StatusUnauthorized, "no session")
		return nil
	}
	return s
}

// isDirectFriend reports whether a and b both have an accepted membership in
// some shared 1:1 ("direct") chat — the new equivalent of the retired
// friends.AreFriends. A friend can always add the other into a group chat
// without the target's allow_chats_from_non_friends opt-in (see handlers.go).
func isDirectFriend(ctx context.Context, a, b int) (bool, error) {
	db, err := pgdb.GetInstanceCtx(ctx)
	if err != nil {
		return false, err
	}
	row, err := db.GetOne(`
		SELECT c.id FROM chats c
		JOIN chat_members m1 ON m1.chat_id = c.id AND m1.user_id = $1 AND m1.status = 'accepted'
		JOIN chat_members m2 ON m2.chat_id = c.id AND m2.user_id = $2 AND m2.status = 'accepted'
		WHERE c.type = 'direct'
		LIMIT 1`, a, b)
	if err != nil {
		return false, err
	}
	return row != nil, nil
}

// allowsChatsFromNonFriends reads one user's own opt-in (see profile.go's
// matching self-service field).
func allowsChatsFromNonFriends(ctx context.Context, userID int) (bool, error) {
	db, err := pgdb.GetInstanceCtx(ctx)
	if err != nil {
		return false, err
	}
	row, err := db.GetOne(`SELECT allow_chats_from_non_friends FROM users WHERE id = $1`, userID)
	if err != nil || row == nil {
		return false, err
	}
	return functions.Truthy(row["allow_chats_from_non_friends"]), nil
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	functions.WriteJSON(w, status, v)
}

func init() {
	module.AddRouteRegistrar(func(router *mux.Router) {
		router.HandleFunc("/api/chats", handleList).Methods("GET")
		router.HandleFunc("/api/chats/direct/{userId}", handleCreateDirect).Methods("POST")
		router.HandleFunc("/api/chats/group", handleCreateGroup).Methods("POST")
		router.HandleFunc("/api/chats/{id}/accept", handleAccept).Methods("POST")
		router.HandleFunc("/api/chats/{id}/decline", handleDecline).Methods("POST")
		router.HandleFunc("/api/chats/{id}/members", handleAddMember).Methods("POST")
		router.HandleFunc("/api/chats/{id}/members/{userId}", handleRemoveMember).Methods("DELETE")
		router.HandleFunc("/api/chats/{id}", handleUpdateChat).Methods("PUT")
		router.HandleFunc("/api/users/search", handleSearch).Methods("GET")
	})
}
