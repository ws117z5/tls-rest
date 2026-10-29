// Package p2prendezvous is the backend half of the standalone P2P messaging
// Android app: a pure rendezvous/"name server" that helps already-online
// clients find each other's address, and never sees message content.
// Membership — who may even ask to connect to whom — is entirely decided by
// go/engine/modules/chats (a persistent chat's accepted members); this
// package only tracks live UDP presence and relays addresses between
// members of a chat the caller already belongs to.
//
//	POST /api/p2p/token              -> {token, ttlSeconds}: mint a short-lived
//	                                     UDP heartbeat token (see tokens.go, udp.go)
//	POST /api/p2p/chats/{id}/connect -> {online: [{id, addr}]}: ask to connect to
//	                                     every other online, accepted member of
//	                                     chat {id}; relays the caller's own address
//	                                     to each of them in turn, so both sides end
//	                                     up with what they need to UDP hole-punch
//	GET  /api/p2p/stream             -> SSE: {event: "chat-event"} pushes from
//	                                     connect, for every chat the caller belongs to
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
	"tls-rest/go/engine/modules/chats"
)

// signalChatPrefix namespaces mesh signal "room" keys per chat id, on the
// shared mesh.QueueSignal/DrainSignal queue other features also use.
const signalChatPrefix = "chat:"

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

// handleConnect asks to connect to every other online, accepted member of
// chat {id} — the caller must be an accepted member themselves. For each
// member currently online, their address is returned to the caller, and the
// caller's own address is pushed to them via the signal queue + hub, so
// every pair ends up with what it needs to UDP hole-punch directly (a full
// mesh for a group chat, same idea at any size).
func handleConnect(w http.ResponseWriter, r *http.Request) {
	s := requireSession(w, r)
	if s == nil {
		return
	}
	chatID, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil || chatID <= 0 {
		functions.JSONError(w, http.StatusBadRequest, "bad chat")
		return
	}

	ok, err := chats.IsAcceptedMember(r.Context(), chatID, s.UserID)
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "query failed")
		return
	}
	if !ok {
		functions.JSONError(w, http.StatusForbidden, "not a member of that chat")
		return
	}

	memberIDs, err := chats.AcceptedMemberIDs(r.Context(), chatID, s.UserID)
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "query failed")
		return
	}

	callerAddr, callerOnline := lookup(s.UserID)
	callerTCPAddr, callerHasTCP := lookupTCP(s.UserID)

	type onlineMember struct {
		UserID  int    `json:"id"`
		Addr    string `json:"addr"`
		TCPAddr string `json:"tcpAddr,omitempty"`
	}
	online := make([]onlineMember, 0, len(memberIDs))
	for _, memberID := range memberIDs {
		addr, isOnline := lookup(memberID)
		if !isOnline {
			continue
		}
		member := onlineMember{UserID: memberID, Addr: addr.String()}
		if tcpAddr, hasTCP := lookupTCP(memberID); hasTCP {
			member.TCPAddr = tcpAddr.String()
		}
		online = append(online, member)

		if callerOnline {
			signalPayload := map[string]interface{}{"userId": s.UserID, "addr": callerAddr.String()}
			if callerHasTCP {
				signalPayload["tcpAddr"] = callerTCPAddr.String()
			}
			payload, _ := json.Marshal(signalPayload)
			mesh.QueueSignal(signalChatPrefix+strconv.FormatInt(chatID, 10), strconv.Itoa(memberID), strconv.Itoa(s.UserID), payload)
			hub.notify(memberID)
		}
	}

	functions.WriteJSON(w, http.StatusOK, map[string]interface{}{"online": online})
}

// handleStream is the caller's one long-lived connection while the Android
// app's foreground service is active: it drains connect-pushed signal events
// (see handleConnect) for every chat the caller currently belongs to.
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

	drainSignals := func() {
		chatIDs, err := chats.ChatIDsForUser(r.Context(), s.UserID)
		if err != nil {
			return
		}
		for _, chatID := range chatIDs {
			room := signalChatPrefix + strconv.FormatInt(chatID, 10)
			for _, msg := range mesh.DrainSignal(room, strconv.Itoa(s.UserID)) {
				var data map[string]interface{}
				if err := json.Unmarshal(msg.Data, &data); err != nil {
					continue
				}
				data["chatId"] = chatID
				send("chat-event", data)
			}
		}
	}

	ch := hub.subscribe(s.UserID)
	defer hub.unsubscribe(s.UserID, ch)

	drainSignals()

	// No chat-membership change can wake this stream on its own (that would mean
	// the UDP hot path or every chats.go write knowing about live subscribers) —
	// a short keep-alive poll is simpler and still near-real-time.
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ch:
			drainSignals()
		case <-ticker.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}

func init() {
	module.AddRouteRegistrar(func(router *mux.Router) {
		router.HandleFunc("/api/p2p/token", handleToken).Methods("POST")
		router.HandleFunc("/api/p2p/chats/{id}/connect", handleConnect).Methods("POST")
		router.HandleFunc("/api/p2p/stream", handleStream).Methods("GET")
	})
	go startUDPListener()
	go startTCPListener()
}
