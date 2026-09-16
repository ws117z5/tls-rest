// Papers-specific wrapper over mesh.QueueSignal/DrainSignal: resolves the
// caller's playerKey and wakes the room's SSE hub.
package papers

import (
	"encoding/json"
	"log"
	"net/http"

	"tls-rest/go/engine/controllers/functions"
	"tls-rest/go/engine/controllers/mesh"

	"github.com/gorilla/mux"
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

	queued := mesh.QueueSignal(roomHash, body.To, from, body.Data)
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

	out := mesh.DrainSignal(roomHash, self)
	if len(out) > 0 {
		log.Printf("[papers/signal] room=%s %s drained %d message(s)", roomHash, self, len(out))
	}

	functions.WriteJSON(w, http.StatusOK, map[string]interface{}{"messages": out})
}
