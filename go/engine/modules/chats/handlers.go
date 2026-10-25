package chats

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"

	"tls-rest/go/engine/controllers/db/pgdb"
	"tls-rest/go/engine/controllers/functions"
)

type memberOut struct {
	UserID   int64  `json:"id"`
	UserName string `json:"userName"`
	Image    string `json:"image"`
	Status   string `json:"status"`
}

type chatOut struct {
	ID       int64       `json:"id"`
	Type     string      `json:"type"`
	Name     string      `json:"name"`
	Avatar   string      `json:"avatar"`
	AdminID  int64       `json:"adminId"`
	MyStatus string      `json:"myStatus"`
	Members  []memberOut `json:"members"`
}

// loadChat assembles one chat's full output, including its member list. Two
// queries (chat row, then members) rather than one join — simplest thing
// that works for a personal messaging app's chat counts, matching how every
// other module this session reads its own small tables.
func loadChat(db *pgdb.Db, chatID int64, callerID int) (*chatOut, error) {
	row, err := db.GetOne(`SELECT id, type, name, avatar, admin_id FROM chats WHERE id = $1`, chatID)
	if err != nil || row == nil {
		return nil, err
	}

	members, err := db.GetAll(`
		SELECT cm.user_id, cm.status, u.user_name, u.image
		FROM chat_members cm JOIN users u ON u.id = cm.user_id
		WHERE cm.chat_id = $1
		ORDER BY cm.joined`, chatID)
	if err != nil {
		return nil, err
	}

	out := &chatOut{
		ID:      chatID,
		Type:    functions.Coerce[string](row["type"]),
		Name:    functions.Coerce[string](row["name"]),
		Avatar:  functions.ImageFieldURL(row["avatar"]),
		AdminID: functions.Coerce[int64](row["admin_id"]),
	}
	for _, m := range members {
		uid := functions.Coerce[int64](m["user_id"])
		status := functions.Coerce[string](m["status"])
		if int(uid) == callerID {
			out.MyStatus = status
		}
		out.Members = append(out.Members, memberOut{
			UserID:   uid,
			UserName: functions.Coerce[string](m["user_name"]),
			Image:    functions.ImageFieldURL(m["image"]),
			Status:   status,
		})
	}
	return out, nil
}

// deleteIfAbandoned removes a chat once it has at most one member left —
// nobody to message, so it's cleaned up rather than left behind forever.
// Covers both "the only other side of a direct chat declined" and "an
// admin-less, member-less group" alike.
func deleteIfAbandoned(db *pgdb.Db, chatID int64) error {
	row, err := db.GetOne(`SELECT COUNT(*) AS n FROM chat_members WHERE chat_id = $1`, chatID)
	if err != nil {
		return err
	}
	if row != nil && functions.Int(row["n"]) <= 1 {
		_, err = db.Exec(`DELETE FROM chats WHERE id = $1`, chatID)
	}
	return err
}

func pathID(r *http.Request, key string) (int64, bool) {
	v, err := strconv.ParseInt(mux.Vars(r)[key], 10, 64)
	return v, err == nil && v > 0
}

// handleList returns every chat the caller is a member of, any status
// (pending ones carry myStatus="pending" so the client can show an
// accept/decline banner instead of a composer).
func handleList(w http.ResponseWriter, r *http.Request) {
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
		SELECT c.id FROM chats c
		JOIN chat_members cm ON cm.chat_id = c.id AND cm.user_id = $1
		ORDER BY c.id DESC`, s.UserID)
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "query failed")
		return
	}

	out := make([]*chatOut, 0, len(rows))
	for _, row := range rows {
		chat, err := loadChat(db, functions.Coerce[int64](row["id"]), s.UserID)
		if err != nil || chat == nil {
			continue
		}
		out = append(out, chat)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"chats": out})
}

// handleCreateDirect creates-or-gets a 1:1 chat with {userId}. No
// friendship/opt-in gate at all — starting a direct chat (i.e. sending a
// friend request) is always allowed; the TARGET decides via accept/decline.
func handleCreateDirect(w http.ResponseWriter, r *http.Request) {
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

	target, err := db.GetOne(`SELECT id FROM users WHERE id = $1`, targetID)
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "query failed")
		return
	}
	if target == nil {
		functions.JSONError(w, http.StatusNotFound, "user not found")
		return
	}

	existing, err := db.GetOne(`
		SELECT c.id FROM chats c
		JOIN chat_members m1 ON m1.chat_id = c.id AND m1.user_id = $1
		JOIN chat_members m2 ON m2.chat_id = c.id AND m2.user_id = $2
		WHERE c.type = 'direct'
		LIMIT 1`, s.UserID, targetID)
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "query failed")
		return
	}

	var chatID int64
	if existing != nil {
		chatID = functions.Coerce[int64](existing["id"])
		// "Adding" someone who already has a pending request open toward the
		// caller is the crossing-in-the-mail case: accept it on the spot
		// rather than leaving two one-sided pending rows.
		if _, err := db.Exec(`
			UPDATE chat_members SET status = 'accepted' WHERE chat_id = $1 AND user_id = $2 AND status = 'pending'`,
			chatID, s.UserID); err != nil {
			functions.JSONError(w, http.StatusInternalServerError, "update failed")
			return
		}
	} else {
		id, err := db.InsertRow("chats", map[string]interface{}{"type": "direct", "admin_id": s.UserID})
		if err != nil {
			functions.JSONError(w, http.StatusInternalServerError, "insert failed")
			return
		}
		chatID = functions.Coerce[int64](id)
		if _, err := db.InsertRow("chat_members", map[string]interface{}{
			"chat_id": chatID, "user_id": s.UserID, "status": "accepted",
		}); err != nil {
			functions.JSONError(w, http.StatusInternalServerError, "insert failed")
			return
		}
		if _, err := db.InsertRow("chat_members", map[string]interface{}{
			"chat_id": chatID, "user_id": targetID, "status": "pending",
		}); err != nil {
			functions.JSONError(w, http.StatusInternalServerError, "insert failed")
			return
		}
	}

	chat, err := loadChat(db, chatID, s.UserID)
	if err != nil || chat == nil {
		functions.JSONError(w, http.StatusInternalServerError, "could not load chat")
		return
	}
	writeJSON(w, http.StatusOK, chat)
}

// handleCreateGroup creates a group chat; the caller is its admin. Each
// requested member is added accepted (if an existing direct-chat friend of
// the caller), pending (if not a friend but they've opted into
// allow_chats_from_non_friends), or skipped (reported back, not a hard
// failure for the rest).
func handleCreateGroup(w http.ResponseWriter, r *http.Request) {
	s := requireSession(w, r)
	if s == nil {
		return
	}
	var body struct {
		Name      string  `json:"name"`
		MemberIDs []int64 `json:"memberIds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		functions.JSONError(w, http.StatusBadRequest, "invalid body")
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" || len(body.MemberIDs) == 0 {
		functions.JSONError(w, http.StatusBadRequest, "name and at least one member are required")
		return
	}

	db, err := pgdb.GetInstanceCtx(r.Context())
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "db unavailable")
		return
	}

	id, err := db.InsertRow("chats", map[string]interface{}{
		"type": "group", "name": body.Name, "admin_id": s.UserID,
	})
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "insert failed")
		return
	}
	chatID := functions.Coerce[int64](id)
	if _, err := db.InsertRow("chat_members", map[string]interface{}{
		"chat_id": chatID, "user_id": s.UserID, "status": "accepted",
	}); err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "insert failed")
		return
	}

	type rejection struct {
		UserID int64  `json:"userId"`
		Reason string `json:"reason"`
	}
	var rejected []rejection
	seen := map[int64]bool{}
	for _, memberID := range body.MemberIDs {
		if memberID == int64(s.UserID) || seen[memberID] {
			continue
		}
		seen[memberID] = true
		status, rejectReason := memberJoinStatus(r.Context(), s.UserID, int(memberID))
		if status == "" {
			rejected = append(rejected, rejection{UserID: memberID, Reason: rejectReason})
			continue
		}
		if _, err := db.InsertRow("chat_members", map[string]interface{}{
			"chat_id": chatID, "user_id": memberID, "status": status,
		}); err != nil {
			rejected = append(rejected, rejection{UserID: memberID, Reason: "could not add"})
		}
	}

	chat, err := loadChat(db, chatID, s.UserID)
	if err != nil || chat == nil {
		functions.JSONError(w, http.StatusInternalServerError, "could not load chat")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"chat": chat, "rejected": rejected})
}

// memberJoinStatus decides how a prospective member joins: accepted if
// they're already a direct-chat friend of the inviter, pending if they've
// opted into allow_chats_from_non_friends, or "" (rejected) otherwise.
func memberJoinStatus(ctx context.Context, inviterID, memberID int) (status, rejectReason string) {
	friend, err := isDirectFriend(ctx, inviterID, memberID)
	if err == nil && friend {
		return "accepted", ""
	}
	allowed, err := allowsChatsFromNonFriends(ctx, memberID)
	if err == nil && allowed {
		return "pending", ""
	}
	return "", "not accepting chats from non-friends"
}

func handleAccept(w http.ResponseWriter, r *http.Request) {
	s := requireSession(w, r)
	if s == nil {
		return
	}
	chatID, ok := pathID(r, "id")
	if !ok {
		functions.JSONError(w, http.StatusBadRequest, "bad chat")
		return
	}
	db, err := pgdb.GetInstanceCtx(r.Context())
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "db unavailable")
		return
	}
	tag, err := db.Exec(`
		UPDATE chat_members SET status = 'accepted' WHERE chat_id = $1 AND user_id = $2 AND status = 'pending'`,
		chatID, s.UserID)
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "update failed")
		return
	}
	if tag.RowsAffected() == 0 {
		functions.JSONError(w, http.StatusNotFound, "no pending invite")
		return
	}
	chat, err := loadChat(db, chatID, s.UserID)
	if err != nil || chat == nil {
		functions.JSONError(w, http.StatusInternalServerError, "could not load chat")
		return
	}
	writeJSON(w, http.StatusOK, chat)
}

func handleDecline(w http.ResponseWriter, r *http.Request) {
	s := requireSession(w, r)
	if s == nil {
		return
	}
	chatID, ok := pathID(r, "id")
	if !ok {
		functions.JSONError(w, http.StatusBadRequest, "bad chat")
		return
	}
	db, err := pgdb.GetInstanceCtx(r.Context())
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "db unavailable")
		return
	}
	if _, err := db.Exec(`DELETE FROM chat_members WHERE chat_id = $1 AND user_id = $2 AND status = 'pending'`,
		chatID, s.UserID); err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "delete failed")
		return
	}
	if err := deleteIfAbandoned(db, chatID); err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "cleanup failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

func requireAdmin(w http.ResponseWriter, db *pgdb.Db, chatID int64, userID int) bool {
	row, err := db.GetOne(`SELECT admin_id FROM chats WHERE id = $1`, chatID)
	if err != nil || row == nil {
		functions.JSONError(w, http.StatusNotFound, "chat not found")
		return false
	}
	if functions.Int(row["admin_id"]) != userID {
		functions.JSONError(w, http.StatusForbidden, "admin only")
		return false
	}
	return true
}

func handleAddMember(w http.ResponseWriter, r *http.Request) {
	s := requireSession(w, r)
	if s == nil {
		return
	}
	chatID, ok := pathID(r, "id")
	if !ok {
		functions.JSONError(w, http.StatusBadRequest, "bad chat")
		return
	}
	var body struct {
		UserID int64 `json:"userId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.UserID <= 0 {
		functions.JSONError(w, http.StatusBadRequest, "invalid body")
		return
	}

	db, err := pgdb.GetInstanceCtx(r.Context())
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "db unavailable")
		return
	}
	if !requireAdmin(w, db, chatID, s.UserID) {
		return
	}

	status, reason := memberJoinStatus(r.Context(), s.UserID, int(body.UserID))
	if status == "" {
		functions.JSONError(w, http.StatusForbidden, reason)
		return
	}
	if _, err := db.InsertRow("chat_members", map[string]interface{}{
		"chat_id": chatID, "user_id": body.UserID, "status": status,
	}); err != nil {
		functions.JSONError(w, http.StatusConflict, "already a member")
		return
	}
	chat, err := loadChat(db, chatID, s.UserID)
	if err != nil || chat == nil {
		functions.JSONError(w, http.StatusInternalServerError, "could not load chat")
		return
	}
	writeJSON(w, http.StatusOK, chat)
}

// handleRemoveMember: the admin can remove anyone; anyone can remove
// themselves (leave).
func handleRemoveMember(w http.ResponseWriter, r *http.Request) {
	s := requireSession(w, r)
	if s == nil {
		return
	}
	chatID, ok := pathID(r, "id")
	targetID, ok2 := pathID(r, "userId")
	if !ok || !ok2 {
		functions.JSONError(w, http.StatusBadRequest, "bad request")
		return
	}

	db, err := pgdb.GetInstanceCtx(r.Context())
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "db unavailable")
		return
	}

	if int(targetID) != s.UserID && !requireAdmin(w, db, chatID, s.UserID) {
		return
	}
	if _, err := db.Exec(`DELETE FROM chat_members WHERE chat_id = $1 AND user_id = $2`, chatID, targetID); err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "delete failed")
		return
	}
	if err := deleteIfAbandoned(db, chatID); err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "cleanup failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

// handleUpdateChat: admin-only rename/re-avatar. The avatar value is
// whatever go/engine/controllers/functions.ImageFieldURL already knows how
// to resolve (a bare hash, or a JSON ref array) — the client uploads through
// the existing POST /api/images/process first, same as the profile avatar.
func handleUpdateChat(w http.ResponseWriter, r *http.Request) {
	s := requireSession(w, r)
	if s == nil {
		return
	}
	chatID, ok := pathID(r, "id")
	if !ok {
		functions.JSONError(w, http.StatusBadRequest, "bad chat")
		return
	}
	var body struct {
		Name   *string `json:"name"`
		Avatar *string `json:"avatar"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		functions.JSONError(w, http.StatusBadRequest, "invalid body")
		return
	}

	db, err := pgdb.GetInstanceCtx(r.Context())
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "db unavailable")
		return
	}
	if !requireAdmin(w, db, chatID, s.UserID) {
		return
	}

	update := map[string]interface{}{}
	if body.Name != nil {
		if name := strings.TrimSpace(*body.Name); name != "" {
			update["name"] = name
		}
	}
	if body.Avatar != nil {
		update["avatar"] = *body.Avatar
	}
	if len(update) > 0 {
		if _, err := db.UpdateRow("chats", update, "id", chatID); err != nil {
			functions.JSONError(w, http.StatusInternalServerError, "update failed")
			return
		}
	}

	chat, err := loadChat(db, chatID, s.UserID)
	if err != nil || chat == nil {
		functions.JSONError(w, http.StatusInternalServerError, "could not load chat")
		return
	}
	writeJSON(w, http.StatusOK, chat)
}
