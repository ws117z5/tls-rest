// Package chats is the persistent replacement for this session's earlier
// friends module and p2prendezvous's code-based rooms: a "chat" (1:1 or
// group) is the only relationship that gates who can message whom. Message
// CONTENT never lives here or anywhere server-side — this package only
// tracks membership/name/avatar/admin; go/engine/modules/p2prendezvous reads
// it to decide who a client may UDP-rendezvous with.
//
//	GET    /api/friends                     -> read straight from the `friends` table — see friends_list.go
//	GET    /api/chats                       -> the caller's chats (any status)
//	POST   /api/chats/direct/{userId}       -> create-or-get a 1:1 chat (send/re-send a friend request)
//	POST   /api/chats/{id}/accept           -> accept caller's own pending membership
//	POST   /api/chats/{id}/decline          -> decline (remove) caller's own pending membership
//	POST   /api/chats/group                 -> {name, memberIds[]}: create a group chat
//	POST   /api/chats/{id}/members          -> admin-only: add one more member (group only)
//	DELETE /api/chats/{id}/members/{userId} -> GROUP only: admin removes anyone, or leave (self)
//	POST   /api/chats/{id}/unfriend         -> DIRECT only: end the friendship, chat stays visible
//	DELETE /api/chats/{id}                  -> DIRECT only: "delete for us" — chat gone for both sides
//	PUT    /api/chats/{id}                  -> admin-only: rename / re-avatar
//	PUT    /api/chats/{id}/settings          -> self-only: show_read_receipts, show_online_status overrides
//	GET    /api/users/search?q=             -> see search.go
//	GET    /api/blocks                      -> who the caller has blocked
//	POST   /api/blocks/{userId}             -> block {userId}: see blocks.go
//	DELETE /api/blocks/{userId}             -> unblock
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

// orderedPair returns (a, b) with the smaller value first — the `friends`
// table's canonical column order (user_a < user_b), so a lookup is always a
// single row read, never an OR across both orderings.
func orderedPair(a, b int) (int, int) {
	if a < b {
		return a, b
	}
	return b, a
}

// isDirectFriend reports whether a and b are friends — a single indexed read
// of the denormalized `friends` table, kept in sync by setFriends below
// rather than recomputed from chat_members on every call (the user's own
// call: a dedicated table is worth the sync bookkeeping here). A friend can
// always add the other into a group chat without the target's
// allow_chats_from_non_friends opt-in (see handlers.go).
func isDirectFriend(ctx context.Context, a, b int) (bool, error) {
	db, err := pgdb.GetInstanceCtx(ctx)
	if err != nil {
		return false, err
	}
	lo, hi := orderedPair(a, b)
	row, err := db.GetOne(`SELECT 1 FROM friends WHERE user_a = $1 AND user_b = $2`, lo, hi)
	if err != nil {
		return false, err
	}
	return row != nil, nil
}

// setFriends is the ONLY place that writes the `friends` table — every
// direct-chat transition that starts or ends a friendship goes through this,
// so chat_members.status and `friends` can't drift apart. Idempotent either
// way (ON CONFLICT DO NOTHING / a DELETE that matches zero rows are both
// fine), so callers don't need to check the current state first.
func setFriends(db *pgdb.Db, a, b int, areFriends bool) error {
	lo, hi := orderedPair(a, b)
	if areFriends {
		_, err := db.Exec(`INSERT INTO friends (user_a, user_b) VALUES ($1, $2) ON CONFLICT (user_a, user_b) DO NOTHING`, lo, hi)
		return err
	}
	_, err := db.Exec(`DELETE FROM friends WHERE user_a = $1 AND user_b = $2`, lo, hi)
	return err
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
		router.HandleFunc("/api/friends", handleListFriends).Methods("GET")
		router.HandleFunc("/api/chats", handleList).Methods("GET")
		router.HandleFunc("/api/chats/direct/{userId}", handleCreateDirect).Methods("POST")
		router.HandleFunc("/api/chats/group", handleCreateGroup).Methods("POST")
		router.HandleFunc("/api/chats/{id}/accept", handleAccept).Methods("POST")
		router.HandleFunc("/api/chats/{id}/decline", handleDecline).Methods("POST")
		router.HandleFunc("/api/chats/{id}/members", handleAddMember).Methods("POST")
		router.HandleFunc("/api/chats/{id}/members/{userId}", handleRemoveMember).Methods("DELETE")
		router.HandleFunc("/api/chats/{id}/unfriend", handleUnfriend).Methods("POST")
		router.HandleFunc("/api/chats/{id}", handleDeleteChat).Methods("DELETE")
		router.HandleFunc("/api/chats/{id}", handleUpdateChat).Methods("PUT")
		router.HandleFunc("/api/chats/{id}/settings", handleUpdateMySettings).Methods("PUT")
		router.HandleFunc("/api/users/search", handleSearch).Methods("GET")
		router.HandleFunc("/api/blocks", handleListBlocks).Methods("GET")
		router.HandleFunc("/api/blocks/{userId}", handleBlock).Methods("POST")
		router.HandleFunc("/api/blocks/{userId}", handleUnblock).Methods("DELETE")
	})
}
