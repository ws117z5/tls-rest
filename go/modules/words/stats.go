package words

import (
	"net/http"
	"time"

	"tls-rest/go/engine/controllers/db/cache"
	"tls-rest/go/engine/controllers/db/pgdb"
	"tls-rest/go/engine/controllers/functions"
)

// statsHandler handles GET /words/stats: the caller's own vocabulary
// totals plus streak/daily-activity, computed fresh on every request (same
// "no caching, plain aggregate SQL" shape as likes.go's loadSummary) —
// there's no reason to precompute this for a single-user query.
func (m *Words) statsHandler(w http.ResponseWriter, r *http.Request) {
	s := cache.SessionFromContext(r.Context())
	if s == nil || s.UserID <= 0 {
		functions.JSONError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	db, err := pgdb.GetInstanceCtx(r.Context())
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "database unavailable")
		return
	}

	totals, err := db.GetOne(`
		SELECT
			COUNT(*) AS words,
			COALESCE(SUM(tries), 0) AS tries,
			COALESCE(SUM(success), 0) AS success,
			COALESCE(SUM(fail), 0) AS fail,
			COUNT(*) FILTER (
				WHERE tries > 0 AND success::float8 / NULLIF(tries, 0) >= 0.8
			) AS mastered
		FROM words
		WHERE created_by = $1
	`, s.UserID)
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	tries := functions.Int(totals["tries"])
	success := functions.Int(totals["success"])
	winRate := 0.0
	if tries > 0 {
		winRate = float64(success) / float64(tries)
	}

	streakDays, practicedToday, err := streak(db, s.UserID)
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	functions.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"words":           functions.Int(totals["words"]),
		"tries":           tries,
		"success":         success,
		"fail":            functions.Int(totals["fail"]),
		"win_rate":        winRate,
		"mastered":        functions.Int(totals["mastered"]),
		"streak_days":     streakDays,
		"practiced_today": practicedToday,
	})
}

// streak walks the caller's distinct practice days (most recent first) and
// counts how many consecutive days, ending today or yesterday, have at
// least one attempt. word_attempts only exists to make this possible — the
// words table's own tries/success/fail are lifetime totals with no dates.
func streak(db *pgdb.Db, userID int) (days int, practicedToday bool, err error) {
	rows, err := db.GetAll(`
		SELECT DISTINCT created::date AS day
		FROM word_attempts
		WHERE created_by = $1
		ORDER BY day DESC
		LIMIT 400
	`, userID)
	if err != nil {
		return 0, false, err
	}
	if len(rows) == 0 {
		return 0, false, nil
	}

	today := time.Now().UTC().Truncate(24 * time.Hour)
	expected := today
	for i, row := range rows {
		day := functions.Coerce[time.Time](row["day"]).UTC().Truncate(24 * time.Hour)
		if i == 0 {
			practicedToday = day.Equal(today)
			if !practicedToday && !day.Equal(today.AddDate(0, 0, -1)) {
				// Most recent attempt isn't today or yesterday — streak is over.
				return 0, false, nil
			}
			expected = day
		}
		if !day.Equal(expected) {
			break
		}
		days++
		expected = expected.AddDate(0, 0, -1)
	}
	return days, practicedToday, nil
}
