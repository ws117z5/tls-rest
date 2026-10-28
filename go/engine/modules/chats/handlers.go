package chats

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"

	"tls-rest/go/engine/controllers/db/pgdb"
	"tls-rest/go/engine/controllers/functions"
)

type memberOut struct {
	UserID   int64  `json:"id"`
	UserName string `json:"userName"`
	Image    string `json:"image"`
	Status   string `json:"status"`
	// FirstName/LastName are blank unless the caller is allowed to see them:
	// always for a direct friend, otherwise only if the member has opted into
	// allow_search_by_name (see profile.go) — same rule as /api/users/search's
	// name-matching, extended to cover display inside a shared chat too.
	FirstName string `json:"firstName,omitempty"`
	LastName  string `json:"lastName,omitempty"`
	// LastSeen (RFC3339) is set only when this member's effective
	// show_online_status (their chat_members override, else their account-wide
	// default — see profile.go/the migration) is true. Omitted entirely otherwise.
	LastSeen string `json:"lastSeen,omitempty"`
}

type chatOut struct {
	ID       int64  `json:"id"`
	Type     string `json:"type"`
	Name     string `json:"name"`
	Avatar   string `json:"avatar"`
	AdminID  int64  `json:"adminId"`
	MyStatus string `json:"myStatus"`
	// The caller's own per-chat overrides (go/engine/modules/chats' PUT
	// .../settings) — never another member's, those are private to them.
	MyShowReadReceipts bool        `json:"myShowReadReceipts"`
	MyShowOnlineStatus string      `json:"myShowOnlineStatus"` // "true" | "false" | "inherit"
	Members            []memberOut `json:"members"`
}

// loadChat assembles one chat's full output, including its member list. Two
// queries (chat row, then members) rather than one join — simplest thing
// that works for a personal messaging app's chat counts, matching how every
// other module this session reads its own small tables.
func loadChat(ctx context.Context, db *pgdb.Db, chatID int64, callerID int) (*chatOut, error) {
	row, err := db.GetOne(`SELECT id, type, name, avatar, admin_id FROM chats WHERE id = $1`, chatID)
	if err != nil || row == nil {
		return nil, err
	}

	members, err := db.GetAll(`
		SELECT cm.user_id, cm.status, cm.show_read_receipts, cm.show_online_status AS chat_show_online,
		       u.user_name, u.image, u.first_name, u.last_name, u.allow_search_by_name,
		       u.show_online_status AS account_show_online, u.last_seen
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
		isCaller := int(uid) == callerID
		if isCaller {
			out.MyStatus = status
			out.MyShowReadReceipts = functions.Truthy(m["show_read_receipts"])
			out.MyShowOnlineStatus = "inherit"
			if m["chat_show_online"] != nil {
				out.MyShowOnlineStatus = strconv.FormatBool(functions.Truthy(m["chat_show_online"]))
			}
		}

		member := memberOut{
			UserID:   uid,
			UserName: functions.Coerce[string](m["user_name"]),
			Image:    functions.ImageFieldURL(m["image"]),
			Status:   status,
		}

		canSeeName := isCaller
		if !canSeeName {
			if friend, _ := isDirectFriend(ctx, callerID, int(uid)); friend {
				canSeeName = true
			} else {
				canSeeName = functions.Truthy(m["allow_search_by_name"])
			}
		}
		if canSeeName {
			member.FirstName = functions.Coerce[string](m["first_name"])
			member.LastName = functions.Coerce[string](m["last_name"])
		}

		showOnline := functions.Truthy(m["account_show_online"])
		if m["chat_show_online"] != nil {
			showOnline = functions.Truthy(m["chat_show_online"])
		}
		if showOnline {
			if ls, ok := m["last_seen"].(time.Time); ok {
				member.LastSeen = ls.UTC().Format(time.RFC3339)
			}
		}

		out.Members = append(out.Members, member)
	}
	return out, nil
}

// deleteIfAbandoned removes a chat once its count of "active" (pending or
// accepted — i.e. excluding 'left'/'unfriended') members drops to
// maxActiveMembers or below. The threshold differs by caller on purpose:
//   - handleDecline passes 1: a direct chat whose invitee just declined has
//     only the original inviter left (never a real two-sided chat), and a
//     group invite that gets declined before anyone else joined is equally
//     pointless — either way, "only the creator, nobody else ever joined"
//     is abandoned.
//   - handleRemoveMember (group leave/kick) passes 0: a group should persist
//     for its last remaining member, not vanish out from under them the
//     moment the second-to-last person leaves — only true emptiness cleans
//     it up.
//
// unfriending/blocking a direct chat never calls this at all — that path
// deliberately keeps the chat row (see handleUnfriend).
func deleteIfAbandoned(db *pgdb.Db, chatID int64, maxActiveMembers int) error {
	row, err := db.GetOne(
		`SELECT COUNT(*) AS n FROM chat_members WHERE chat_id = $1 AND status IN ('pending', 'accepted')`, chatID)
	if err != nil {
		return err
	}
	if row != nil && functions.Int(row["n"]) <= maxActiveMembers {
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

	// A 'left' group chat is hidden from the leaver's own list (the chat
	// continues for everyone else); an 'unfriended' direct chat stays visible
	// to both sides — see the status comment on chat_members in the migration.
	rows, err := db.GetAll(`
		SELECT c.id FROM chats c
		JOIN chat_members cm ON cm.chat_id = c.id AND cm.user_id = $1 AND cm.status <> 'left'
		ORDER BY c.id DESC`, s.UserID)
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "query failed")
		return
	}

	out := make([]*chatOut, 0, len(rows))
	for _, row := range rows {
		chat, err := loadChat(r.Context(), db, functions.Coerce[int64](row["id"]), s.UserID)
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

	blocked, err := isBlocked(r.Context(), s.UserID, int(targetID))
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "query failed")
		return
	}
	if blocked {
		functions.JSONError(w, http.StatusForbidden, "blocked")
		return
	}

	existing, err := db.GetOne(`
		SELECT c.id, m1.status AS caller_status, m2.status AS target_status
		FROM chats c
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
		callerStatus := functions.Coerce[string](existing["caller_status"])
		targetStatus := functions.Coerce[string](existing["target_status"])
		switch {
		case callerStatus == "pending":
			// Crossing in the mail: {userId} already had a pending invite open
			// toward the caller — accept it on the spot rather than leaving two
			// one-sided pending rows. Both sides are accepted now: friends.
			if _, err := db.Exec(`UPDATE chat_members SET status = 'accepted' WHERE chat_id = $1 AND user_id = $2`,
				chatID, s.UserID); err != nil {
				functions.JSONError(w, http.StatusInternalServerError, "update failed")
				return
			}
			if err := setFriends(db, s.UserID, int(targetID), true); err != nil {
				functions.JSONError(w, http.StatusInternalServerError, "update failed")
				return
			}
		case callerStatus == "unfriended" || targetStatus == "unfriended":
			// Re-friending after an explicit unfriend: a fresh request cycle,
			// reusing the same chat row rather than creating a new one.
			if _, err := db.Exec(`UPDATE chat_members SET status = 'accepted' WHERE chat_id = $1 AND user_id = $2`,
				chatID, s.UserID); err != nil {
				functions.JSONError(w, http.StatusInternalServerError, "update failed")
				return
			}
			if _, err := db.Exec(`UPDATE chat_members SET status = 'pending' WHERE chat_id = $1 AND user_id = $2`,
				chatID, targetID); err != nil {
				functions.JSONError(w, http.StatusInternalServerError, "update failed")
				return
			}
		}
		// Otherwise: already accepted, or caller already has a pending invite
		// open toward {userId} — nothing to do, just return current state.
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

	chat, err := loadChat(r.Context(), db, chatID, s.UserID)
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

	chat, err := loadChat(r.Context(), db, chatID, s.UserID)
	if err != nil || chat == nil {
		functions.JSONError(w, http.StatusInternalServerError, "could not load chat")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"chat": chat, "rejected": rejected})
}

// memberJoinStatus decides how a prospective member joins: rejected outright
// if either side has blocked the other, accepted if they're already a
// direct-chat friend of the inviter, pending if they've opted into
// allow_chats_from_non_friends, or "" (rejected) otherwise.
func memberJoinStatus(ctx context.Context, inviterID, memberID int) (status, rejectReason string) {
	if blocked, err := isBlocked(ctx, inviterID, memberID); err == nil && blocked {
		return "", "blocked"
	}
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

	// For a DIRECT chat, the inviter's own row is always already 'accepted'
	// (set at creation/re-add) — the invitee accepting here is the moment
	// both sides are accepted, i.e. the moment they become friends.
	chatRow, err := db.GetOne(`SELECT type FROM chats WHERE id = $1`, chatID)
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "query failed")
		return
	}
	if chatRow != nil && functions.Coerce[string](chatRow["type"]) == "direct" {
		other, err := db.GetOne(`SELECT user_id FROM chat_members WHERE chat_id = $1 AND user_id <> $2`, chatID, s.UserID)
		if err != nil {
			functions.JSONError(w, http.StatusInternalServerError, "query failed")
			return
		}
		if other != nil {
			if err := setFriends(db, s.UserID, functions.Int(other["user_id"]), true); err != nil {
				functions.JSONError(w, http.StatusInternalServerError, "update failed")
				return
			}
		}
	}

	chat, err := loadChat(r.Context(), db, chatID, s.UserID)
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
	if err := deleteIfAbandoned(db, chatID, 1); err != nil {
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
	chat, err := loadChat(r.Context(), db, chatID, s.UserID)
	if err != nil || chat == nil {
		functions.JSONError(w, http.StatusInternalServerError, "could not load chat")
		return
	}
	writeJSON(w, http.StatusOK, chat)
}

// handleRemoveMember: GROUP chats only — the admin can remove anyone; anyone
// can remove themselves (leave). For a direct (1:1) chat, see handleUnfriend
// instead: "leave" and "unfriend" are deliberately two separate actions, not
// one reused for both (a 1:1 chat has no group to keep existing for).
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

	chatRow, err := db.GetOne(`SELECT type FROM chats WHERE id = $1`, chatID)
	if err != nil || chatRow == nil {
		functions.JSONError(w, http.StatusNotFound, "chat not found")
		return
	}
	if functions.Coerce[string](chatRow["type"]) != "group" {
		functions.JSONError(w, http.StatusBadRequest, "use /unfriend for a direct chat")
		return
	}

	if int(targetID) != s.UserID && !requireAdmin(w, db, chatID, s.UserID) {
		return
	}
	if _, err := db.Exec(`UPDATE chat_members SET status = 'left' WHERE chat_id = $1 AND user_id = $2`,
		chatID, targetID); err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "update failed")
		return
	}
	if err := deleteIfAbandoned(db, chatID, 0); err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "cleanup failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

// handleUnfriend ends a direct (1:1) chat's friend relationship WITHOUT
// deleting the chat — both members flip to 'unfriended' (see the status
// comment on chat_members in the migration). The chat stays visible to both
// sides; re-adding (POST /api/chats/direct/{userId}) starts a fresh request
// cycle reusing the same chat row.
func handleUnfriend(w http.ResponseWriter, r *http.Request) {
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

	chatRow, err := db.GetOne(`SELECT type FROM chats WHERE id = $1`, chatID)
	if err != nil || chatRow == nil {
		functions.JSONError(w, http.StatusNotFound, "chat not found")
		return
	}
	if functions.Coerce[string](chatRow["type"]) != "direct" {
		functions.JSONError(w, http.StatusBadRequest, "unfriend only applies to a direct chat")
		return
	}
	isMember, err := IsAcceptedMember(r.Context(), chatID, s.UserID)
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "query failed")
		return
	}
	if !isMember {
		functions.JSONError(w, http.StatusForbidden, "not a member of that chat")
		return
	}

	other, err := db.GetOne(`SELECT user_id FROM chat_members WHERE chat_id = $1 AND user_id <> $2`, chatID, s.UserID)
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "query failed")
		return
	}
	if _, err := db.Exec(`UPDATE chat_members SET status = 'unfriended' WHERE chat_id = $1`, chatID); err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "update failed")
		return
	}
	if other != nil {
		if err := setFriends(db, s.UserID, functions.Int(other["user_id"]), false); err != nil {
			functions.JSONError(w, http.StatusInternalServerError, "update failed")
			return
		}
	}
	chat, err := loadChat(r.Context(), db, chatID, s.UserID)
	if err != nil || chat == nil {
		functions.JSONError(w, http.StatusInternalServerError, "could not load chat")
		return
	}
	writeJSON(w, http.StatusOK, chat)
}

// handleDeleteChat is "delete for us": a DIRECT chat only, stronger than
// unfriend — the chat row itself is removed (chat_members cascades via its
// FK), not just marked 'unfriended', so it's gone from both sides entirely
// rather than staying visible. "Delete for me" (the other half of what the
// client calls these two actions) never reaches the server at all — it's
// purely local, hiding the chat on one device without touching anyone else's
// copy or the friendship.
func handleDeleteChat(w http.ResponseWriter, r *http.Request) {
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

	chatRow, err := db.GetOne(`SELECT type FROM chats WHERE id = $1`, chatID)
	if err != nil || chatRow == nil {
		functions.JSONError(w, http.StatusNotFound, "chat not found")
		return
	}
	if functions.Coerce[string](chatRow["type"]) != "direct" {
		functions.JSONError(w, http.StatusBadRequest, "delete-for-us only applies to a direct chat")
		return
	}
	isMember, err := IsAcceptedMember(r.Context(), chatID, s.UserID)
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "query failed")
		return
	}
	if !isMember {
		functions.JSONError(w, http.StatusForbidden, "not a member of that chat")
		return
	}

	other, err := db.GetOne(`SELECT user_id FROM chat_members WHERE chat_id = $1 AND user_id <> $2`, chatID, s.UserID)
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "query failed")
		return
	}
	if _, err := db.Exec(`DELETE FROM chats WHERE id = $1`, chatID); err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "delete failed")
		return
	}
	if other != nil {
		if err := setFriends(db, s.UserID, functions.Int(other["user_id"]), false); err != nil {
			functions.JSONError(w, http.StatusInternalServerError, "update failed")
			return
		}
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

	chat, err := loadChat(r.Context(), db, chatID, s.UserID)
	if err != nil || chat == nil {
		functions.JSONError(w, http.StatusInternalServerError, "could not load chat")
		return
	}
	writeJSON(w, http.StatusOK, chat)
}

// handleUpdateMySettings: self-only (never the admin, never another member) —
// each member's own privacy knobs for this one chat. show_read_receipts is a
// plain bool; show_online_status is a tri-state string ("true" | "false" |
// "inherit") rather than bool-or-null, so the request body never needs an
// actual JSON null to mean "clear the override" (Gson on the Android side
// omits null map values by default, which would make that otherwise
// indistinguishable from "don't touch this field").
func handleUpdateMySettings(w http.ResponseWriter, r *http.Request) {
	s := requireSession(w, r)
	if s == nil {
		return
	}
	chatID, ok := pathID(r, "id")
	if !ok {
		functions.JSONError(w, http.StatusBadRequest, "bad chat")
		return
	}

	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		functions.JSONError(w, http.StatusBadRequest, "invalid body")
		return
	}

	db, err := pgdb.GetInstanceCtx(r.Context())
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "db unavailable")
		return
	}

	isMember, err := IsAcceptedMember(r.Context(), chatID, s.UserID)
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "query failed")
		return
	}
	if !isMember {
		functions.JSONError(w, http.StatusForbidden, "not a member of that chat")
		return
	}

	if v, present := body["show_read_receipts"]; present {
		if _, err := db.Exec(`UPDATE chat_members SET show_read_receipts = $1 WHERE chat_id = $2 AND user_id = $3`,
			functions.Truthy(v), chatID, s.UserID); err != nil {
			functions.JSONError(w, http.StatusInternalServerError, "update failed")
			return
		}
	}
	if v, present := body["show_online_status"]; present {
		str, _ := v.(string)
		var value interface{}
		switch str {
		case "true":
			value = true
		case "false":
			value = false
		case "inherit", "":
			value = nil
		default:
			functions.JSONError(w, http.StatusBadRequest, `show_online_status must be "true", "false", or "inherit"`)
			return
		}
		if _, err := db.Exec(`UPDATE chat_members SET show_online_status = $1 WHERE chat_id = $2 AND user_id = $3`,
			value, chatID, s.UserID); err != nil {
			functions.JSONError(w, http.StatusInternalServerError, "update failed")
			return
		}
	}

	chat, err := loadChat(r.Context(), db, chatID, s.UserID)
	if err != nil || chat == nil {
		functions.JSONError(w, http.StatusInternalServerError, "could not load chat")
		return
	}
	writeJSON(w, http.StatusOK, chat)
}
