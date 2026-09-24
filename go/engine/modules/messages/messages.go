// Package messages provides direct messaging between two users. A message
// has a sender and a recipient (unlike comments, which are polymorphic over
// module/row); a thread is just every message between two user ids, either
// direction. Read and written over a small REST API:
//
//	GET  /api/messages/inbox          -> one row per conversation partner
//	GET  /api/messages/thread/{id}    -> the full thread with user {id}
//	POST /api/messages/thread/{id}    -> send a message to user {id}
//
// Display names in this package are deliberately the username only (never
// first/last name), matching the public profile's privacy rule (see
// engine/modules/profile) — a conversation partner is identified the same
// limited way a stranger's profile is. The standalone "messages" CRUD module
// is for admin cleanup, same shape as engine/modules/comments.
package messages

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"tls-rest/go/app"
	"tls-rest/go/engine/controllers/db/cache"
	"tls-rest/go/engine/controllers/db/pgdb"
	"tls-rest/go/engine/controllers/field"
	"tls-rest/go/engine/controllers/functions"
	"tls-rest/go/engine/controllers/module"

	"github.com/gorilla/mux"
)

// Messages module, backs ConversationList/MessageThread; ID avoids clashing with the inbox page name.
var Module = &module.ModuleAbstract[interface{}]{
	ID:      "message_log",
	Name:    "Message Log",
	Icon:    "messages",
	Submenu: "engine",
	Fields: []field.Field{
		field.NewField("sender_id", field.TYPE_INT, true).WithLabel("Sender"),
		field.NewField("recipient_id", field.TYPE_INT, true).WithLabel("Recipient"),
		field.NewField("body", field.TYPE_TEXT, true).WithLabel("Message"),
	},
	DefaultPermission:    module.PERMISSION_DENY,
	DefaultPermissionSet: true,
	Rights:               make(map[int]int),
	CustomRoutes: []module.CustomRoute{
		{Path: "/api/messages/inbox", Methods: []string{"GET"}, Handler: handleInbox, Absolute: true},
		{Path: "/api/messages/unread-count", Methods: []string{"GET"}, Handler: handleUnreadCount, Absolute: true},
		{Path: "/api/messages/thread/{id}", Methods: []string{"GET"}, Handler: handleThread, Absolute: true},
		{Path: "/api/messages/thread/{id}", Methods: []string{"POST"}, Handler: handleSend, Absolute: true},
	},
}

func init() {
	app.RegisterModule(Module, "messages")
}

type conversation struct {
	UserID      int64       `json:"userId"`
	UserName    string      `json:"userName"`
	Image       string      `json:"image"`
	LastBody    string      `json:"lastBody"`
	LastCreated interface{} `json:"lastCreated"`
	Unread      int         `json:"unread"`
}

type message struct {
	ID      int64       `json:"id"`
	Body    string      `json:"body"`
	Created interface{} `json:"created"`
	// Mine is true when the caller sent this message — computed server-side
	// so the frontend never needs to know its own numeric user id.
	Mine bool `json:"mine"`
}

func requireSession(w http.ResponseWriter, r *http.Request) *cache.Session {
	s := cache.SessionFromContext(r.Context())
	if s == nil || s.UserID <= 0 {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return nil
	}
	return s
}

// handleInbox lists every conversation the caller is part of, most recent
// message first, with a per-conversation unread count.
func handleInbox(w http.ResponseWriter, r *http.Request) {
	s := requireSession(w, r)
	if s == nil {
		return
	}

	db, err := pgdb.GetInstanceCtx(r.Context())
	if err != nil {
		http.Error(w, "db unavailable", http.StatusInternalServerError)
		return
	}

	rows, err := db.GetAll(`
		WITH convo AS (
			SELECT CASE WHEN sender_id = $1 THEN recipient_id ELSE sender_id END AS other_id,
			       body, created
			FROM messages
			WHERE sender_id = $1 OR recipient_id = $1
		), ranked AS (
			SELECT other_id, body, created,
			       ROW_NUMBER() OVER (PARTITION BY other_id ORDER BY created DESC) AS rn
			FROM convo
		)
		SELECT r.other_id, u.user_name, u.image, r.body AS last_body, r.created AS last_created,
		       (SELECT COUNT(*) FROM messages m2
		        WHERE m2.recipient_id = $1 AND m2.sender_id = r.other_id AND m2.read_at IS NULL) AS unread
		FROM ranked r
		JOIN users u ON u.id = r.other_id
		WHERE r.rn = 1
		ORDER BY r.created DESC`, s.UserID)
	if err != nil {
		http.Error(w, "query failed", http.StatusInternalServerError)
		return
	}

	out := make([]conversation, 0, len(rows))
	for _, row := range rows {
		out = append(out, conversation{
			UserID:      functions.Coerce[int64](row["other_id"]),
			UserName:    functions.Coerce[string](row["user_name"]),
			Image:       functions.ImageFieldURL(row["image"]),
			LastBody:    functions.Coerce[string](row["last_body"]),
			LastCreated: row["last_created"],
			Unread:      functions.Int(row["unread"]),
		})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"conversations": out})
}

func unreadCount(r *http.Request, userID int) (int, error) {
	db, err := pgdb.GetInstanceCtx(r.Context())
	if err != nil {
		return 0, err
	}
	row, err := db.GetOne(
		`SELECT COUNT(*) AS n FROM messages WHERE recipient_id = $1 AND read_at IS NULL`, userID,
	)
	if err != nil {
		return 0, err
	}
	return functions.Int(row["n"]), nil
}

// handleUnreadCount streams the caller's unread count for the menu badge: one
// event on connect, then one whenever handleSend/handleThread change it —
// replacing what used to be a 30s poll (see hub.go).
func handleUnreadCount(w http.ResponseWriter, r *http.Request) {
	s := requireSession(w, r)
	if s == nil {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	// Opt this stream out of the server's blanket write-timeout, same as any
	// other long-lived SSE connection (see papers.GameEvents).
	_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch := hub.subscribe(s.UserID)
	defer hub.unsubscribe(s.UserID, ch)

	send := func() bool {
		n, err := unreadCount(r, s.UserID)
		if err != nil {
			return false
		}
		fmt.Fprintf(w, "data: %d\n\n", n)
		flusher.Flush()
		return true
	}
	if !send() {
		return
	}

	keep := time.NewTicker(25 * time.Second)
	defer keep.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ch:
			if !send() {
				return
			}
		case <-keep.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}

// handleThread returns every message between the caller and {id}, oldest
// first, and marks the caller's incoming messages in it as read.
func handleThread(w http.ResponseWriter, r *http.Request) {
	s := requireSession(w, r)
	if s == nil {
		return
	}

	otherID, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil || otherID <= 0 {
		http.Error(w, "bad user", http.StatusBadRequest)
		return
	}

	db, err := pgdb.GetInstanceCtx(r.Context())
	if err != nil {
		http.Error(w, "db unavailable", http.StatusInternalServerError)
		return
	}

	rows, err := db.GetAll(`
		SELECT id, sender_id, body, created FROM messages
		WHERE (sender_id = $1 AND recipient_id = $2) OR (sender_id = $2 AND recipient_id = $1)
		ORDER BY created`, s.UserID, otherID)
	if err != nil {
		http.Error(w, "query failed", http.StatusInternalServerError)
		return
	}

	out := make([]message, 0, len(rows))
	for _, row := range rows {
		out = append(out, message{
			ID:      functions.Coerce[int64](row["id"]),
			Body:    functions.Coerce[string](row["body"]),
			Created: row["created"],
			Mine:    functions.Int(row["sender_id"]) == s.UserID,
		})
	}

	// Best-effort: mark what the caller just read as read. A failure here
	// shouldn't stop the thread from rendering.
	tag, _ := db.Exec(
		`UPDATE messages SET read_at = now() WHERE recipient_id = $1 AND sender_id = $2 AND read_at IS NULL`,
		s.UserID, otherID,
	)
	if tag.RowsAffected() > 0 {
		hub.notify(s.UserID) // the caller's own unread count just dropped
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"messages": out})
}

// handleSend appends one message from the caller to {id}.
func handleSend(w http.ResponseWriter, r *http.Request) {
	s := requireSession(w, r)
	if s == nil {
		return
	}

	otherID, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil || otherID <= 0 {
		http.Error(w, "bad user", http.StatusBadRequest)
		return
	}
	if otherID == s.UserID {
		http.Error(w, "cannot message yourself", http.StatusBadRequest)
		return
	}

	var in struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	body := strings.TrimSpace(in.Body)
	if body == "" {
		http.Error(w, "empty message", http.StatusBadRequest)
		return
	}

	db, err := pgdb.GetInstanceCtx(r.Context())
	if err != nil {
		http.Error(w, "db unavailable", http.StatusInternalServerError)
		return
	}

	target, err := db.GetOne(`SELECT id FROM users WHERE id = $1`, otherID)
	if err != nil {
		http.Error(w, "query failed", http.StatusInternalServerError)
		return
	}
	if target == nil {
		http.Error(w, "recipient not found", http.StatusNotFound)
		return
	}

	id, err := db.InsertRow("messages", map[string]interface{}{
		"sender_id":    s.UserID,
		"recipient_id": otherID,
		"body":         body,
		"created_by":   s.UserID,
	})
	if err != nil {
		http.Error(w, "insert failed", http.StatusInternalServerError)
		return
	}
	hub.notify(otherID)

	writeJSON(w, http.StatusOK, map[string]interface{}{"id": id})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
