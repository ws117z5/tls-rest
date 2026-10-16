package events

import "sync"

// dueHub fans out "your due-notification count changed" pings to a user's open
// SSE streams. Carries no data — subscribers re-query their own count on a
// ping. Same shape as messages/hub.go's unreadHub.
type dueHub struct {
	mu   sync.Mutex
	subs map[int]map[chan struct{}]bool // userID -> subscriber channels
}

var hub = &dueHub{subs: map[int]map[chan struct{}]bool{}}

func (h *dueHub) subscribe(userID int) chan struct{} {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.subs[userID] == nil {
		h.subs[userID] = map[chan struct{}]bool{}
	}
	ch := make(chan struct{}, 4)
	h.subs[userID][ch] = true
	return ch
}

func (h *dueHub) unsubscribe(userID int, ch chan struct{}) {
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

// notify pings all of userID's open streams (non-blocking).
func (h *dueHub) notify(userID int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs[userID] {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}
