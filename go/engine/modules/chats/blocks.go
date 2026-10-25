package chats

import (
	"context"
	"net/http"

	"tls-rest/go/engine/controllers/db/pgdb"
	"tls-rest/go/engine/controllers/functions"
)

// isBlocked reports whether a has blocked b, OR b has blocked a — blocking
// is checked both directions everywhere it's used (createDirect,
// memberJoinStatus, search), so a blocked relationship is symmetric in
// effect even though the blocks table itself records who blocked whom.
func isBlocked(ctx context.Context, a, b int) (bool, error) {
	db, err := pgdb.GetInstanceCtx(ctx)
	if err != nil {
		return false, err
	}
	row, err := db.GetOne(`
		SELECT 1 FROM blocks
		WHERE (blocker_id = $1 AND blocked_id = $2) OR (blocker_id = $2 AND blocked_id = $1)
		LIMIT 1`, a, b)
	if err != nil {
		return false, err
	}
	return row != nil, nil
}

// handleBlock blocks {userId}: records the block and, if an accepted direct
// chat exists between the two, ends it the same way an explicit unfriend
// does (both members -> 'unfriended') — chat stays visible, just no longer
// "friends", consistent with handleUnfriend rather than a separate rule.
func handleBlock(w http.ResponseWriter, r *http.Request) {
	s := requireSession(w, r)
	if s == nil {
		return
	}
	targetID, ok := pathID(r, "userId")
	if !ok || int(targetID) == s.UserID {
		functions.JSONError(w, http.StatusBadRequest, "bad user")
		return
	}

	db, err := pgdb.GetInstanceCtx(r.Context())
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "db unavailable")
		return
	}

	if _, err := db.Exec(`
		INSERT INTO blocks (blocker_id, blocked_id) VALUES ($1, $2)
		ON CONFLICT (blocker_id, blocked_id) DO NOTHING`, s.UserID, targetID); err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "insert failed")
		return
	}

	existing, err := db.GetOne(`
		SELECT c.id FROM chats c
		JOIN chat_members m1 ON m1.chat_id = c.id AND m1.user_id = $1
		JOIN chat_members m2 ON m2.chat_id = c.id AND m2.user_id = $2
		WHERE c.type = 'direct'
		LIMIT 1`, s.UserID, targetID)
	if err == nil && existing != nil {
		_, _ = db.Exec(`UPDATE chat_members SET status = 'unfriended' WHERE chat_id = $1`,
			functions.Coerce[int64](existing["id"]))
		_ = setFriends(db, s.UserID, int(targetID), false)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

func handleUnblock(w http.ResponseWriter, r *http.Request) {
	s := requireSession(w, r)
	if s == nil {
		return
	}
	targetID, ok := pathID(r, "userId")
	if !ok {
		functions.JSONError(w, http.StatusBadRequest, "bad user")
		return
	}
	db, err := pgdb.GetInstanceCtx(r.Context())
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "db unavailable")
		return
	}
	if _, err := db.Exec(`DELETE FROM blocks WHERE blocker_id = $1 AND blocked_id = $2`, s.UserID, targetID); err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "delete failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

type blockedUserOut struct {
	ID       int64  `json:"id"`
	UserName string `json:"userName"`
	Image    string `json:"image"`
}

// handleListBlocks returns everyone the caller has blocked (not who has
// blocked the caller — that's never exposed to the blocked person).
func handleListBlocks(w http.ResponseWriter, r *http.Request) {
	s := requireSession(w, r)
	if s == nil {
		return
	}
	db, err := pgdb.GetInstanceCtx(r.Context())
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "db unavailable")
		return
	}
	rows, err := db.GetAll(`
		SELECT u.id, u.user_name, u.image FROM blocks b
		JOIN users u ON u.id = b.blocked_id
		WHERE b.blocker_id = $1
		ORDER BY u.user_name`, s.UserID)
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "query failed")
		return
	}
	out := make([]blockedUserOut, 0, len(rows))
	for _, row := range rows {
		out = append(out, blockedUserOut{
			ID:       functions.Coerce[int64](row["id"]),
			UserName: functions.Coerce[string](row["user_name"]),
			Image:    functions.ImageFieldURL(row["image"]),
		})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"blocked": out})
}
