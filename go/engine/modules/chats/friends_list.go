package chats

import (
	"net/http"

	"tls-rest/go/engine/controllers/db/pgdb"
	"tls-rest/go/engine/controllers/functions"
)

type friendOut struct {
	ID       int64  `json:"id"`
	UserName string `json:"userName"`
	Image    string `json:"image"`
}

// handleListFriends reads straight from the `friends` table (not derived
// from chats/chat_members at request time) — the Android Find People screen
// uses this for its default "Friends" list, reserving /api/chats for actual
// chat metadata/membership state.
func handleListFriends(w http.ResponseWriter, r *http.Request) {
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
		SELECT u.id, u.user_name, u.image
		FROM friends f
		JOIN users u ON u.id = CASE WHEN f.user_a = $1 THEN f.user_b ELSE f.user_a END
		WHERE f.user_a = $1 OR f.user_b = $1
		ORDER BY u.user_name`, s.UserID)
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "query failed")
		return
	}

	out := make([]friendOut, 0, len(rows))
	for _, row := range rows {
		out = append(out, friendOut{
			ID:       functions.Coerce[int64](row["id"]),
			UserName: functions.Coerce[string](row["user_name"]),
			Image:    functions.ImageFieldURL(row["image"]),
		})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"friends": out})
}
