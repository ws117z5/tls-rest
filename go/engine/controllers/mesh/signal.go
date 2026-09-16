// Generic per-room signaling queue, wrapped by each module's own HTTP handlers.
package mesh

import (
	"encoding/json"
	"sync"
)

type SignalMessage struct {
	From string          `json:"from"`
	Data json.RawMessage `json:"data"`
}

var (
	signalMu    sync.Mutex
	signalQueue = map[string]map[string][]SignalMessage{} // room -> recipient -> queued messages
)

// QueueSignal queues one signaling message for `to` in `room`, tagged with the
// sender's identity, and returns the number of messages now pending for `to`.
func QueueSignal(room, to, from string, data json.RawMessage) int {
	signalMu.Lock()
	defer signalMu.Unlock()
	r, ok := signalQueue[room]
	if !ok {
		r = map[string][]SignalMessage{}
		signalQueue[room] = r
	}
	r[to] = append(r[to], SignalMessage{From: from, Data: data})
	return len(r[to])
}

// DrainSignal returns and clears every message queued for `self` in `room`.
func DrainSignal(room, self string) []SignalMessage {
	signalMu.Lock()
	defer signalMu.Unlock()
	var out []SignalMessage
	if r, ok := signalQueue[room]; ok {
		out = r[self]
		delete(r, self)
	}
	if out == nil {
		out = []SignalMessage{}
	}
	return out
}
