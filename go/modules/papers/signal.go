// Signaling relay for the room's WebRTC mesh (see js/src/modules/papers/
// videoMesh.ts, which drives lib/mesh.ts's MeshManager). A peer POSTs an SDP
// offer/answer or ICE candidate addressed to another peer; it sits in an
// in-memory per-room, per-recipient queue until that peer drains it. Delivery
// piggybacks on the room's existing SSE hub (hub.go) — a send pings every
// subscriber, exactly like a turn/state change does, so the recipient's next
// "changed" event prompts it to drain its queue.
package papers

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"tls-rest/go/engine/controllers/functions"

	"github.com/gorilla/mux"
)

type signalMsg struct {
	From string          `json:"from"`
	Data json.RawMessage `json:"data"`
}

var (
	signalMu    sync.Mutex
	signalQueue = map[string]map[string][]signalMsg{} // room hash -> recipient key -> queued messages
)

// SendSignal handles POST /papers/{roomId}/game/signal {to, data}: queues one
// signaling message for `to`, tagged with the sender's identity, and wakes the
// room's SSE subscribers.
func SendSignal(w http.ResponseWriter, r *http.Request) {
	roomHash := mux.Vars(r)["roomId"]
	from := playerKey(r)
	if from == "" {
		functions.JSONError(w, http.StatusUnauthorized, "no session")
		return
	}

	var body struct {
		To   string          `json:"to"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.To == "" {
		functions.JSONError(w, http.StatusBadRequest, "invalid signal")
		return
	}

	signalMu.Lock()
	room, ok := signalQueue[roomHash]
	if !ok {
		room = map[string][]signalMsg{}
		signalQueue[roomHash] = room
	}
	room[body.To] = append(room[body.To], signalMsg{From: from, Data: body.Data})
	queued := len(room[body.To])
	signalMu.Unlock()

	log.Printf("[papers/signal] room=%s %s -> %s queued (pending for recipient: %d)", roomHash, from, body.To, queued)

	hub.notify(roomHash) // same "something changed" ping the turn/state handlers use
	functions.WriteJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

// DrainSignals handles GET /papers/{roomId}/game/signal: returns and clears
// every message queued for the caller in this room.
func DrainSignals(w http.ResponseWriter, r *http.Request) {
	roomHash := mux.Vars(r)["roomId"]
	self := playerKey(r)
	if self == "" {
		functions.JSONError(w, http.StatusUnauthorized, "no session")
		return
	}

	signalMu.Lock()
	var out []signalMsg
	if room, ok := signalQueue[roomHash]; ok {
		out = room[self]
		delete(room, self)
	}
	signalMu.Unlock()
	if out == nil {
		out = []signalMsg{}
	}
	if len(out) > 0 {
		log.Printf("[papers/signal] room=%s %s drained %d message(s)", roomHash, self, len(out))
	}

	functions.WriteJSON(w, http.StatusOK, map[string]interface{}{"messages": out})
}
