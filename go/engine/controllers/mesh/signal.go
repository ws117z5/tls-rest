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

// RegisterSignalQueuedHook adds a callback fired after every QueueSignal
// call with (room, to) — same callback-injection idiom as
// module.OnRightsChange/OnConfigChange, used so this package can wake a
// recipient's own SSE/poll stream without importing the modules that own
// those streams (which would cycle back here). More than one module reacts
// — p2prendezvous wakes a per-user stream keyed by a numeric `to`, papers
// wakes a per-room stream keyed by `room` — so this is a list, not a single
// assignable var: each module's own init() registers its own hook, and
// none of them need to know about, or run in any order relative to, the
// others. A hook that doesn't recognize `room`/`to` as its own is expected
// to just no-op. The server's own in-process signal handling (relaynode)
// drains in-process and needs no hook here at all.
var signalQueuedHooks []func(room, to string)

func RegisterSignalQueuedHook(fn func(room, to string)) {
	signalQueuedHooks = append(signalQueuedHooks, fn)
}

// QueueSignal queues one signaling message for `to` in `room`, tagged with the
// sender's identity, and returns the number of messages now pending for `to`.
func QueueSignal(room, to, from string, data json.RawMessage) int {
	signalMu.Lock()
	r, ok := signalQueue[room]
	if !ok {
		r = map[string][]SignalMessage{}
		signalQueue[room] = r
	}
	r[to] = append(r[to], SignalMessage{From: from, Data: data})
	n := len(r[to])
	signalMu.Unlock()

	for _, fn := range signalQueuedHooks {
		fn(room, to)
	}
	return n
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
