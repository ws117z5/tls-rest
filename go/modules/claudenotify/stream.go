package claudenotify

import (
	"fmt"
	"net/http"
	"time"

	"tls-rest/go/engine/controllers/db/cache"
)

// handleStream handles GET /api/claude-notifications/stream: a content-free
// SSE ping (same shape as events.handleDueCount) telling the client a new
// notification landed — the client re-lists GET /claude_notifications itself.
func handleStream(w http.ResponseWriter, r *http.Request) {
	s := cache.SessionFromContext(r.Context())
	if s == nil || s.UserID <= 0 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch := hub.subscribe(s.UserID)
	defer hub.unsubscribe(s.UserID, ch)

	fmt.Fprint(w, "event: ready\ndata: {}\n\n")
	flusher.Flush()

	keep := time.NewTicker(25 * time.Second)
	defer keep.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ch:
			fmt.Fprint(w, "data: changed\n\n")
			flusher.Flush()
		case <-keep.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}
