// Package config resolves per-user configuration from three levels — global,
// group, and user — with user overriding group overriding global (over built-in
// defaults). Config is stored as named columns (one per parameter) so it is set
// as a small form (theme select, date format string), not a generic key/value.
// Resolved config is cached on the session and recomputed only when the config
// epoch is bumped (a config row changed) or the session is recreated.
package config

import (
	"context"
	"encoding/json"
	"sync/atomic"

	"tls-rest/go/engine/controllers/db/pgdb"
)

// Parameter keys (also the config table column names).
const (
	KeyTheme      = "theme"
	KeyDateFormat = "date_format"
	// KeySyncWithGoogleDefault is the default state of a new event's own
	// "sync with Google Calendar" checkbox (events.sync_with_google), stored
	// as text "true"/"false" like the other scalar keys — see
	// go/engine/pages/profile/profile.go.
	KeySyncWithGoogleDefault = "sync_with_google_default"
	// KeyModuleViews holds the resolved module:mode -> view-name map, JSON-encoded as an object
	// (e.g. {"posts:list":"gallery"}) — AppConfig.moduleView parses it client-side. Not a plain scalar
	// like the keys above, since module_views stores one row per (module, mode) pair (see moduleViewRow).
	KeyModuleViews = "module_views"
)

// Defaults are used when nothing is configured at any level.
var Defaults = map[string]string{
	KeyTheme:                 "light",      // "light" | "dark"
	KeyDateFormat:            "YYYY-MM-DD", // display date format string
	KeySyncWithGoogleDefault: "true",
}

var configEpoch int64

func CurrentConfigEpoch() int64 { return atomic.LoadInt64(&configEpoch) }
func BumpConfigEpoch()          { atomic.AddInt64(&configEpoch, 1) }

// moduleViewRow is one row of a config record's module_views table field (go/engine/modules/config):
// "use the view named View for module Module in mode Mode", e.g. {"posts","list","gallery"}.
type moduleViewRow struct {
	Module string `json:"module"`
	Mode   string `json:"mode"`
	View   string `json:"view"`
}

// mergeModuleViews decodes one config row's module_views column (a JSON array of moduleViewRow, or empty/nil) and
// writes its entries into dst keyed "module:mode" — called in priority order (global, group, user) so a later call
// overwrites only the specific pairs it declares, leaving the rest of dst (set by an earlier, lower-priority level)
// alone, the same per-key-override behaviour the scalar fields below get for free from a single column.
func mergeModuleViews(dst map[string]string, raw interface{}) {
	var rows []moduleViewRow
	switch v := raw.(type) {
	case string:
		if v == "" {
			return
		}
		if json.Unmarshal([]byte(v), &rows) != nil {
			return
		}
	case []byte:
		if len(v) == 0 {
			return
		}
		if json.Unmarshal(v, &rows) != nil {
			return
		}
	case []interface{}, []map[string]interface{}:
		// pgx decodes a jsonb column into native Go values, not the raw JSON
		// text — round-trip through JSON to reuse moduleViewRow's own decoding
		// instead of hand-walking maps.
		b, err := json.Marshal(v)
		if err != nil {
			return
		}
		if json.Unmarshal(b, &rows) != nil {
			return
		}
	default:
		return
	}
	for _, r := range rows {
		if r.Module == "" || r.Mode == "" || r.View == "" {
			continue
		}
		dst[r.Module+":"+r.Mode] = r.View
	}
}

// Resolve returns the effective config for a user: defaults, then global, then
// the user's group(s), then the user's own row — later levels overriding earlier
// ones, per non-empty column. userID <= 0 resolves defaults + global only.
func Resolve(ctx context.Context, userID int) map[string]string {
	out := make(map[string]string, len(Defaults))
	for k, v := range Defaults {
		out[k] = v
	}

	db, err := pgdb.GetInstanceCtx(ctx)
	if err != nil {
		return out
	}

	moduleViews := map[string]string{}
	apply := func(rows []map[string]interface{}) {
		for _, r := range rows {
			if t, _ := r[KeyTheme].(string); t != "" {
				out[KeyTheme] = t
			}
			if d, _ := r[KeyDateFormat].(string); d != "" {
				out[KeyDateFormat] = d
			}
			if sw, _ := r[KeySyncWithGoogleDefault].(string); sw != "" {
				out[KeySyncWithGoogleDefault] = sw
			}
			mergeModuleViews(moduleViews, r[KeyModuleViews])
		}
	}

	// Global.
	if rows, e := db.RQuery(
		`SELECT theme, date_format, sync_with_google_default, module_views FROM config WHERE scope = 'global'`,
	); e == nil {
		apply(rows)
	}

	if userID > 0 {
		// Group level, across every group the user belongs to (higher id wins).
		if rows, e := db.RQuery(`
			SELECT theme, date_format, sync_with_google_default, module_views FROM config
			WHERE scope = 'group' AND scope_id IN (
				SELECT jsonb_array_elements_text(groups)::int FROM users WHERE id = $1
			)
			ORDER BY scope_id ASC
		`, userID); e == nil {
			apply(rows)
		}
		// User level — highest priority.
		if rows, e := db.RQuery(
			`SELECT theme, date_format, sync_with_google_default, module_views FROM config WHERE scope = 'user' AND scope_id = $1`,
			userID,
		); e == nil {
			apply(rows)
		}
	}

	if len(moduleViews) > 0 {
		if b, err := json.Marshal(moduleViews); err == nil {
			out[KeyModuleViews] = string(b)
		}
	}

	return out
}
