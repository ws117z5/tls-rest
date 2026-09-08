package papers

import (
	"sync"
	"time"
)

// gameHub fans out "state changed" pings to every SSE subscriber of a room. It
// carries no data — clients pull their own personalised /game/state on a ping,
// so a player's own assigned word is never sent to their browser.
type gameHub struct {
	mu   sync.Mutex
	subs map[string]map[chan struct{}]bool // roomUUID -> subscriber channels
}

var hub = &gameHub{subs: map[string]map[chan struct{}]bool{}}

func (h *gameHub) subscribe(room string) chan struct{} {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.subs[room] == nil {
		h.subs[room] = map[chan struct{}]bool{}
	}
	ch := make(chan struct{}, 4)
	h.subs[room][ch] = true
	return ch
}

func (h *gameHub) unsubscribe(room string, ch chan struct{}) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.subs[room] != nil {
		delete(h.subs[room], ch)
		if len(h.subs[room]) == 0 {
			delete(h.subs, room)
		}
	}
	close(ch)
}

// notify pings all subscribers of a room (non-blocking).
func (h *gameHub) notify(room string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs[room] {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// scheduleExpiry fires once at the turn deadline: it expires the active turn (if
// still running) and pings clients, so a naturally-timed-out turn updates
// instantly without anyone polling. Harmless if the turn already ended (guarded
// by expireTurn's deadline check).
func scheduleExpiry(room string, secs int) {
	time.AfterFunc(time.Duration(secs)*time.Second, func() {
		st, _ := GetRoomState(room)
		expireTurn(&st)
		SetRoomState(st)
		hub.notify(room)
	})
}
