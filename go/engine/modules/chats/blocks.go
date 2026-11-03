package chats

import (
	"context"
	"encoding/json"
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

// hasOpenReport reports whether userID currently has an open (uncleared)
// report against them — checked in handleCreateDirect to gate a reported
// user out of sending any new friend requests until an admin flips the
// report's status to 'cleared' through the generic reports module.
func hasOpenReport(ctx context.Context, userID int) (bool, error) {
	db, err := pgdb.GetInstanceCtx(ctx)
	if err != nil {
		return false, err
	}
	row, err := db.GetOne(`SELECT 1 FROM reports WHERE reported_id = $1 AND status = 'open' LIMIT 1`, userID)
	if err != nil {
		return false, err
	}
	return row != nil, nil
}

// blockUser is the shared core of both handleBlock and handleReportUser:
// records the block, drops any subscription between the two (either
// direction — a block ends subscriber-only post visibility too, not just
// friendship), and, if an accepted direct chat exists between the two, ends
// it the same way an explicit unfriend does (both members -> 'unfriended')
// — chat stays visible, just no longer "friends".
func blockUser(db *pgdb.Db, blockerID, blockedID int) error {
	if _, err := db.Exec(`
		INSERT INTO blocks (blocker_id, blocked_id) VALUES ($1, $2)
		ON CONFLICT (blocker_id, blocked_id) DO NOTHING`, blockerID, blockedID); err != nil {
		return err
	}
	if err := setSubscribed(db, blockerID, blockedID, false); err != nil {
		return err
	}
	if err := setSubscribed(db, blockedID, blockerID, false); err != nil {
		return err
	}

	existing, err := db.GetOne(`
		SELECT c.id FROM chats c
		JOIN chat_members m1 ON m1.chat_id = c.id AND m1.user_id = $1
		JOIN chat_members m2 ON m2.chat_id = c.id AND m2.user_id = $2
		WHERE c.type = 'direct'
		LIMIT 1`, blockerID, blockedID)
	if err == nil && existing != nil {
		_, _ = db.Exec(`UPDATE chat_members SET status = 'unfriended' WHERE chat_id = $1`,
			functions.Coerce[int64](existing["id"]))
		_ = setFriends(db, blockerID, blockedID, false)
	}
	return nil
}

// handleBlock blocks {userId} — see blockUser.
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

	if err := blockUser(db, s.UserID, int(targetID)); err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "update failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

// handleReportUser is "Block and report": does everything handleBlock does,
// plus records a reports row carrying whatever message {userId} attached to
// their most recent request toward the caller (see handleCreateDirect) — an
// optional {message} in the body overrides that when there's nothing to
// pull from (e.g. reporting someone with no pending request). An open
// report blocks {userId} from sending any new friend requests (see
// hasOpenReport) until an admin clears it via the generic reports module.
func handleReportUser(w http.ResponseWriter, r *http.Request) {
	s := requireSession(w, r)
	if s == nil {
		return
	}
	targetID, ok := pathID(r, "userId")
	if !ok || int(targetID) == s.UserID {
		functions.JSONError(w, http.StatusBadRequest, "bad user")
		return
	}
	var body struct {
		Message string `json:"message"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	db, err := pgdb.GetInstanceCtx(r.Context())
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "db unavailable")
		return
	}

	message := body.Message
	if msgRow, err := db.GetOne(`
		SELECT message FROM chat_members
		WHERE user_id = $1 AND message IS NOT NULL AND message <> ''
		  AND chat_id IN (SELECT chat_id FROM chat_members WHERE user_id = $2)
		ORDER BY joined DESC LIMIT 1`, targetID, s.UserID); err == nil && msgRow != nil {
		if stored := functions.Coerce[string](msgRow["message"]); stored != "" {
			message = stored
		}
	}

	if err := blockUser(db, s.UserID, int(targetID)); err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "update failed")
		return
	}
	if _, err := db.InsertRow("reports", map[string]interface{}{
		"reporter_id": s.UserID, "reported_id": targetID, "message": message, "status": "open",
	}); err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "insert failed")
		return
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
