package p2prendezvous

import "sync"

// hub fans out "something changed, re-check" pings to a user's open
// /api/p2p/stream connections — the exact pattern engine/modules/messages
// uses for its unread-count stream. Carries no data: a ping just means
// "drain your queued signals and re-push your online-friends snapshot".
type notifyHub struct {
	mu   sync.Mutex
	subs map[int]map[chan struct{}]bool
}

var hub = &notifyHub{subs: map[int]map[chan struct{}]bool{}}

func (h *notifyHub) subscribe(userID int) chan struct{} {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.subs[userID] == nil {
		h.subs[userID] = map[chan struct{}]bool{}
	}
	ch := make(chan struct{}, 4)
	h.subs[userID][ch] = true
	return ch
}

func (h *notifyHub) unsubscribe(userID int, ch chan struct{}) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.subs[userID] != nil {
		delete(h.subs[userID], ch)
		if len(h.subs[userID]) == 0 {
			delete(h.subs, userID)
		}
	}
	close(ch)
}

func (h *notifyHub) notify(userID int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs[userID] {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}
