// Package p2prendezvous is the backend half of the standalone P2P messaging
// Android app: a pure rendezvous/"name server" that helps already-online
// clients find each other's address, and never sees message content. Two
// ways to connect, with different authorization:
//
//   - 1:1, friends-gated: POST /api/p2p/connect/{id} only works between
//     accepted friends (see go/engine/modules/friends).
//
//   - group, NOT friends-gated: rooms (rooms.go, rooms_handlers.go) let any
//     number of authenticated users exchange messages, friends or not —
//     knowing/choosing the room's code is the only authorization. Every
//     member connects directly to every other member (a full mesh), so
//     "many clients get the same message" is the sender fanning its
//     message out to each peer it holds a direct link to, not a server
//     relay.
//
//     POST /api/p2p/token           -> {token, ttlSeconds}: mint a short-lived
//     UDP heartbeat token (see tokens.go, udp.go)
//     POST /api/p2p/connect/{id}    -> {online, addr?}: ask to connect to friend
//     {id}; if they're online, relays each
//     side's observed address to the other
//     POST /api/p2p/rooms           -> {code, members}: create+join a new room
//     with a fresh random code
//     POST /api/p2p/rooms/{code}/join  -> {code, members}: join an existing
//     (or not-yet-existing) room by code
//     POST /api/p2p/rooms/{code}/leave -> {ok}: leave a room
//     GET  /api/p2p/stream          -> SSE: {event: "online-friends"} snapshots,
//     {event: "connect-request"} (1:1), and
//     {event: "room-event"} (room join/leave)
//
// Actual messages travel directly between clients over UDP once they have
// each other's address — this package is never in that path.
package p2prendezvous

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"

	"tls-rest/go/engine/controllers/db/cache"
	"tls-rest/go/engine/controllers/functions"
	"tls-rest/go/engine/controllers/mesh"
	"tls-rest/go/engine/controllers/module"
	"tls-rest/go/engine/modules/friends"
)

// signalRoom is the single fixed "room" passed to the generic mesh signal
// queue (see engine/controllers/mesh/signal.go) — this package has no
// per-conversation rooms, just one global connect-request channel keyed by
// recipient user id.
const signalRoom = "p2p-connect"

// onlineSnapshotInterval is how often an open /api/p2p/stream re-checks and
// re-pushes which of the caller's friends are online. There's no per-friend
// fan-out of presence changes (that would mean the UDP hot path querying the
// friends graph on every single heartbeat) — a short poll inside the stream
// is far simpler and still near-real-time.
const onlineSnapshotInterval = 8 * time.Second

func requireSession(w http.ResponseWriter, r *http.Request) *cache.Session {
	s := cache.SessionFromContext(r.Context())
	if s == nil || s.UserID <= 0 {
		functions.JSONError(w, http.StatusUnauthorized, "no session")
		return nil
	}
	return s
}

// handleToken mints a fresh UDP heartbeat token for the caller.
func handleToken(w http.ResponseWriter, r *http.Request) {
	s := requireSession(w, r)
	if s == nil {
		return
	}
	token := mintToken(s.UserID)
	functions.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"token":      token,
		"ttlSeconds": int(tokenTTL.Seconds()),
	})
}

// handleConnect asks to connect to friend {id}: friends-gated, and only
// succeeds if that friend currently has a live UDP heartbeat. On success it
// hands the caller the friend's address, and pushes the caller's own address
// to the friend via the signal queue + hub, so both sides end up with what
// they need to start UDP hole-punching toward each other.
func handleConnect(w http.ResponseWriter, r *http.Request) {
	s := requireSession(w, r)
	if s == nil {
		return
	}
	friendID, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil || friendID <= 0 {
		functions.JSONError(w, http.StatusBadRequest, "bad user")
		return
	}

	ok, err := friends.AreFriends(r.Context(), s.UserID, friendID)
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "query failed")
		return
	}
	if !ok {
		functions.JSONError(w, http.StatusForbidden, "not friends")
		return
	}

	friendAddr, online := lookup(friendID)
	if !online {
		functions.WriteJSON(w, http.StatusOK, map[string]interface{}{"online": false})
		return
	}

	if callerAddr, ok := lookup(s.UserID); ok {
		payload, _ := json.Marshal(map[string]string{"addr": callerAddr.String()})
		mesh.QueueSignal(signalRoom, strconv.Itoa(friendID), strconv.Itoa(s.UserID), payload)
		hub.notify(friendID)
	}

	functions.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"online": true,
		"addr":   friendAddr.String(),
	})
}

// handleStream is the caller's one long-lived connection while the Android
// app's foreground service is active: it carries connect-request pushes
// (see handleConnect) and periodic online-friends snapshots.
func handleStream(w http.ResponseWriter, r *http.Request) {
	s := requireSession(w, r)
	if s == nil {
		return
	}
	flusher, isFlusher := w.(http.Flusher)
	if !isFlusher {
		functions.JSONError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	send := func(event string, payload interface{}) {
		data, err := json.Marshal(payload)
		if err != nil {
			return
		}
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
		flusher.Flush()
	}

	sendOnlineSnapshot := func() {
		friendIDs, err := friends.ListFriendIDs(r.Context(), s.UserID)
		if err != nil {
			return
		}
		online := make([]int, 0, len(friendIDs))
		for _, id := range friendIDs {
			if isOnline(id) {
				online = append(online, id)
			}
		}
		send("online-friends", map[string]interface{}{"online": online})
	}

	drainSignals := func() {
		for _, msg := range mesh.DrainSignal(signalRoom, strconv.Itoa(s.UserID)) {
			var data map[string]string
			if err := json.Unmarshal(msg.Data, &data); err != nil {
				continue
			}
			send("connect-request", map[string]interface{}{"from": msg.From, "addr": data["addr"]})
		}
		for _, code := range roomsForUser(s.UserID) {
			for _, msg := range mesh.DrainSignal(signalRoomPrefix+code, strconv.Itoa(s.UserID)) {
				var data map[string]interface{}
				if err := json.Unmarshal(msg.Data, &data); err != nil {
					continue
				}
				data["room"] = code
				send("room-event", data)
			}
		}
	}

	ch := hub.subscribe(s.UserID)
	defer hub.unsubscribe(s.UserID, ch)

	sendOnlineSnapshot()
	drainSignals()

	ticker := time.NewTicker(onlineSnapshotInterval)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ch:
			drainSignals()
			sendOnlineSnapshot()
		case <-ticker.C:
			sendOnlineSnapshot()
		}
	}
}

func init() {
	module.AddRouteRegistrar(func(router *mux.Router) {
		router.HandleFunc("/api/p2p/token", handleToken).Methods("POST")
		router.HandleFunc("/api/p2p/connect/{id}", handleConnect).Methods("POST")
		router.HandleFunc("/api/p2p/stream", handleStream).Methods("GET")
		router.HandleFunc("/api/p2p/rooms", handleCreateRoom).Methods("POST")
		router.HandleFunc("/api/p2p/rooms/{code}/join", handleJoinRoom).Methods("POST")
		router.HandleFunc("/api/p2p/rooms/{code}/leave", handleLeaveRoom).Methods("POST")
	})
	go startUDPListener()
}
