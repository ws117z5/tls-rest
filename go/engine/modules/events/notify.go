package events

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"tls-rest/go/engine/controllers/db/cache"
	"tls-rest/go/engine/controllers/db/pgdb"
	"tls-rest/go/engine/controllers/functions"
)

// visibilityACL mirrors fieldset_engine.buildVisibilityCondition's ACL branch
// for this module (created_by / visible_users / visible_groups / global) —
// written out by hand here because the due-count/notifier queries run outside
// the generic fieldset engine (no list request, no viewer object to build).
const visibilityACL = `(
	created_by = $1
	OR EXISTS (SELECT 1 FROM jsonb_array_elements_text(visible_users) AS vu WHERE vu::int = $1)
	OR EXISTS (
		SELECT 1 FROM jsonb_array_elements_text(visible_groups) AS vg
		WHERE vg::int IN (SELECT jsonb_array_elements_text(u.groups)::int FROM users u WHERE u.id = $1)
	)
	OR scope = 'global'
)`

// dueCount returns how many of userID's events have a reminder due right now
// and not yet notified.
func dueCount(ctx context.Context, userID int) (int, error) {
	db, err := pgdb.GetInstanceCtx(ctx)
	if err != nil {
		return 0, err
	}
	// notify_offset_minutes is declared TYPE_SELECT (a dropdown of integer
	// values), but the engine has no TYPE_SELECT case in fieldTypeToSQL and
	// falls back to TEXT — hence the ::int cast, not because the stored value
	// is ever actually non-numeric.
	row, err := db.GetOne(`
		SELECT COUNT(*) AS n FROM events
		WHERE notify = true AND notified_at IS NULL
		  AND starts_at - (notify_offset_minutes::int * interval '1 minute') <= now()
		  AND `+visibilityACL+`
	`, userID)
	if err != nil {
		return 0, err
	}
	return functions.Int(row["n"]), nil
}

func requireSession(w http.ResponseWriter, r *http.Request) *cache.Session {
	s := cache.SessionFromContext(r.Context())
	if s == nil || s.UserID <= 0 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return nil
	}
	return s
}

// handleDueCount handles GET /api/events/due-count: streams the caller's due
// reminder count for the menu badge, same shape as messages.handleUnreadCount.
func handleDueCount(w http.ResponseWriter, r *http.Request) {
	s := requireSession(w, r)
	if s == nil {
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

	// A query failure skips this tick rather than closing the stream — the
	// connection should stay open as long as the client does; a transient DB
	// hiccup shouldn't force the browser into an EventSource reconnect loop
	// (which looks like "the connection keeps reopening" from the outside).
	send := func() {
		n, err := dueCount(r.Context(), s.UserID)
		if err != nil {
			return
		}
		fmt.Fprintf(w, "data: %d\n\n", n)
		flusher.Flush()
	}
	send()

	keep := time.NewTicker(25 * time.Second)
	defer keep.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ch:
			send()
		case <-keep.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}
