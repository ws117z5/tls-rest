package p2prendezvous

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	"tls-rest/go/engine/controllers/functions"
	"tls-rest/go/engine/controllers/mesh"
)

// handleCreateRoom mints a fresh random room code and joins the caller to
// it — a convenience over POSTing /api/p2p/rooms/{code}/join with a code
// you picked yourself (which also works, e.g. a shared memorable
// passphrase a group of non-friends agrees on out of band).
func handleCreateRoom(w http.ResponseWriter, r *http.Request) {
	s := requireSession(w, r)
	if s == nil {
		return
	}
	joinRoomAndRespond(w, s.UserID, generateRoomCode())
}

// handleJoinRoom adds the caller to room {code}. No friendship check at
// all, unlike /api/p2p/connect/{id} — knowing the code is the only
// authorization a room needs.
func handleJoinRoom(w http.ResponseWriter, r *http.Request) {
	s := requireSession(w, r)
	if s == nil {
		return
	}
	code := mux.Vars(r)["code"]
	if code == "" {
		functions.JSONError(w, http.StatusBadRequest, "bad room code")
		return
	}
	joinRoomAndRespond(w, s.UserID, code)
}

// joinRoomAndRespond does the actual join: responds with every other
// current member's id+address (if they're online) so the caller can start
// hole-punching to all of them immediately, and pushes a "joined" room
// event to each of those members so they learn about the caller in turn —
// this is what makes a room a full mesh rather than just a list.
func joinRoomAndRespond(w http.ResponseWriter, userID int, code string) {
	others := joinRoom(code, userID)
	callerAddr, callerOnline := lookup(userID)

	type member struct {
		UserID int    `json:"userId"`
		Addr   string `json:"addr"`
	}
	members := make([]member, 0, len(others))
	for _, otherID := range others {
		addr, online := lookup(otherID)
		if !online {
			continue
		}
		members = append(members, member{UserID: otherID, Addr: addr.String()})

		if callerOnline {
			notifyRoomEvent(code, otherID, "joined", userID, callerAddr.String())
		}
	}

	functions.WriteJSON(w, http.StatusOK, map[string]interface{}{"code": code, "members": members})
}

// handleLeaveRoom removes the caller from room {code} and lets the
// remaining members know, so they can drop a peer that isn't coming back
// instead of retrying it forever.
func handleLeaveRoom(w http.ResponseWriter, r *http.Request) {
	s := requireSession(w, r)
	if s == nil {
		return
	}
	code := mux.Vars(r)["code"]
	if code == "" {
		functions.JSONError(w, http.StatusBadRequest, "bad room code")
		return
	}
	for _, otherID := range leaveRoom(code, s.UserID) {
		notifyRoomEvent(code, otherID, "left", s.UserID, "")
	}
	functions.WriteJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

// notifyRoomEvent queues a {type, userId, addr?} payload for `to` on room
// code's own namespaced mesh signal queue, and wakes their SSE stream.
func notifyRoomEvent(code string, to int, eventType string, userID int, addr string) {
	payload := map[string]interface{}{"type": eventType, "userId": userID}
	if addr != "" {
		payload["addr"] = addr
	}
	data, _ := json.Marshal(payload)
	mesh.QueueSignal(signalRoomPrefix+code, strconv.Itoa(to), strconv.Itoa(userID), data)
	hub.notify(to)
}
