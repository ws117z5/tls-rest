package chats

import (
	"context"

	"tls-rest/go/engine/controllers/db/pgdb"
	"tls-rest/go/engine/controllers/functions"
)

// IsAcceptedMember and AcceptedMemberIDs are exported for
// go/engine/modules/p2prendezvous, which gates UDP rendezvous on chat
// membership instead of the retired friends table.

// IsAcceptedMember reports whether userID has an accepted (not pending)
// membership in chatID.
func IsAcceptedMember(ctx context.Context, chatID int64, userID int) (bool, error) {
	db, err := pgdb.GetInstanceCtx(ctx)
	if err != nil {
		return false, err
	}
	row, err := db.GetOne(
		`SELECT 1 FROM chat_members WHERE chat_id = $1 AND user_id = $2 AND status = 'accepted'`,
		chatID, userID)
	if err != nil {
		return false, err
	}
	return row != nil, nil
}

// AcceptedMemberIDs returns every other accepted member of chatID (excluding excludeUserID).
func AcceptedMemberIDs(ctx context.Context, chatID int64, excludeUserID int) ([]int, error) {
	db, err := pgdb.GetInstanceCtx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := db.GetAll(
		`SELECT user_id FROM chat_members WHERE chat_id = $1 AND status = 'accepted' AND user_id <> $2`,
		chatID, excludeUserID)
	if err != nil {
		return nil, err
	}
	out := make([]int, 0, len(rows))
	for _, row := range rows {
		out = append(out, functions.Int(row["user_id"]))
	}
	return out, nil
}

// ChatIDsForUser returns every chat userID is an accepted member of — what
// the p2prendezvous SSE stream drains signal events for.
func ChatIDsForUser(ctx context.Context, userID int) ([]int64, error) {
	db, err := pgdb.GetInstanceCtx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := db.GetAll(
		`SELECT chat_id FROM chat_members WHERE user_id = $1 AND status = 'accepted'`,
		userID)
	if err != nil {
		return nil, err
	}
	out := make([]int64, 0, len(rows))
	for _, row := range rows {
		out = append(out, functions.Coerce[int64](row["chat_id"]))
	}
	return out, nil
}
