// Package config resolves per-user configuration from three levels — global,
// group, and user — with user overriding group overriding global (over built-in
// defaults). Config is stored as named columns (one per parameter) so it is set
// as a small form (theme select, date format string), not a generic key/value.
// Resolved config is cached on the session and recomputed only when the config
// epoch is bumped (a config row changed) or the session is recreated.
package config

import (
	"sync/atomic"

	"tls-rest/go/engine/controllers/db/pgdb"
)

// Parameter keys (also the config table column names).
const (
	KeyTheme      = "theme"
	KeyDateFormat = "date_format"
)

// Defaults are used when nothing is configured at any level.
var Defaults = map[string]string{
	KeyTheme:      "light",      // "light" | "dark"
	KeyDateFormat: "YYYY-MM-DD", // display date format string
}

var configEpoch int64

func CurrentConfigEpoch() int64 { return atomic.LoadInt64(&configEpoch) }
func BumpConfigEpoch()          { atomic.AddInt64(&configEpoch, 1) }

// Resolve returns the effective config for a user: defaults, then global, then
// the user's group(s), then the user's own row — later levels overriding earlier
// ones, per non-empty column. userID <= 0 resolves defaults + global only.
func Resolve(userID int) map[string]string {
	out := make(map[string]string, len(Defaults))
	for k, v := range Defaults {
		out[k] = v
	}

	db, err := pgdb.GetInstance()
	if err != nil {
		return out
	}

	apply := func(rows []map[string]interface{}) {
		for _, r := range rows {
			if t, _ := r[KeyTheme].(string); t != "" {
				out[KeyTheme] = t
			}
			if d, _ := r[KeyDateFormat].(string); d != "" {
				out[KeyDateFormat] = d
			}
		}
	}

	// Global.
	if rows, e := db.RQuery(
		`SELECT theme, date_format FROM config WHERE scope = 'global'`,
	); e == nil {
		apply(rows)
	}

	if userID > 0 {
		// Group level, across every group the user belongs to (higher id wins).
		if rows, e := db.RQuery(`
			SELECT theme, date_format FROM config
			WHERE scope = 'group' AND scope_id IN (
				SELECT user_group FROM users WHERE id = $1 AND user_group IS NOT NULL
				UNION
				SELECT group_id FROM user_group_members WHERE user_id = $1
			)
			ORDER BY scope_id ASC
		`, userID); e == nil {
			apply(rows)
		}
		// User level — highest priority.
		if rows, e := db.RQuery(
			`SELECT theme, date_format FROM config WHERE scope = 'user' AND scope_id = $1`,
			userID,
		); e == nil {
			apply(rows)
		}
	}

	return out
}
