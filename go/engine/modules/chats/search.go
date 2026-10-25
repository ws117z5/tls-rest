package chats

import (
	"net/http"
	"strings"

	"tls-rest/go/engine/controllers/db/pgdb"
	"tls-rest/go/engine/controllers/functions"
)

type searchResult struct {
	ID       int64  `json:"id"`
	UserName string `json:"userName"`
	Image    string `json:"image"`
}

// handleSearch matches username always; first/last name only for users who
// opted into allow_search_by_name (see profile.go) — never email, never
// anything else about the matched users.
func handleSearch(w http.ResponseWriter, r *http.Request) {
	s := requireSession(w, r)
	if s == nil {
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeJSON(w, http.StatusOK, map[string]interface{}{"users": []searchResult{}})
		return
	}

	db, err := pgdb.GetInstanceCtx(r.Context())
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "db unavailable")
		return
	}

	rows, err := db.GetAll(`
		SELECT id, user_name, image FROM users
		WHERE id <> $1 AND (
			user_name ILIKE '%' || $2 || '%'
			OR (allow_search_by_name = true AND (first_name ILIKE '%' || $2 || '%' OR last_name ILIKE '%' || $2 || '%'))
		)
		ORDER BY user_name
		LIMIT 25`, s.UserID, q)
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "query failed")
		return
	}

	out := make([]searchResult, 0, len(rows))
	for _, row := range rows {
		out = append(out, searchResult{
			ID:       functions.Coerce[int64](row["id"]),
			UserName: functions.Coerce[string](row["user_name"]),
			Image:    functions.ImageFieldURL(row["image"]),
		})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"users": out})
}
