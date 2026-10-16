// Package events is a scheduling module: title/time/duration/repeat events
// with a per-row visibility scope (self/group/global) resolved from the
// creator's role, an optional reminder (notify + notify_offset_minutes), and
// optional one-way push sync to the creator's Google Calendar
// (sync_with_google — see go/engine/pages/profile/googlepush.go). The
// Calendar page (go/pages/calendar) reads and writes events through this
// module's ordinary CRUD API — it has no backend of its own.
package events

import (
	"context"
	"net/http"
	"time"

	"tls-rest/go/app"
	"tls-rest/go/engine/controllers/db/cache"
	"tls-rest/go/engine/controllers/db/pgdb"
	. "tls-rest/go/engine/controllers/field"
	"tls-rest/go/engine/controllers/functions"
	. "tls-rest/go/engine/controllers/module"
	"tls-rest/go/engine/pages/profile"

	"github.com/gorilla/mux"
)

// notifyOffsetOptions lists the reminder lead times a user may pick.
func notifyOffsetOptions() []map[string]interface{} {
	return []map[string]interface{}{
		{"value": 5, "name": "5 minutes before"},
		{"value": 10, "name": "10 minutes before"},
		{"value": 15, "name": "15 minutes before"},
		{"value": 30, "name": "30 minutes before"},
		{"value": 60, "name": "1 hour before"},
	}
}

// repeatOptions lists the recurrence patterns an event may have. Translated
// to a Google Calendar RRULE when pushed (see googlepush.go's rruleFor) and
// used to re-arm a local reminder for its next occurrence once it fires
// (see notifier.go's rollForward).
func repeatOptions() []map[string]interface{} {
	return []map[string]interface{}{
		{"value": "none", "name": "Does not repeat"},
		{"value": "daily", "name": "Daily"},
		{"value": "weekly", "name": "Weekly"},
		{"value": "monthly", "name": "Monthly"},
		{"value": "annual", "name": "Annually"},
	}
}

// scopeOptions lists the scopes a viewer may assign to an event they create:
// a plain user or moderator never elevates past "self"; only group_admin and
// full admin get any scope control at all. Mirrored server-side by
// enforceScopeAuthority below, since the options list alone is a UI
// convenience, not an authority boundary — a crafted request could submit
// any value regardless of what this offers.
func scopeOptions(viewer map[string]interface{}) []map[string]interface{} {
	self := map[string]interface{}{"value": "self", "name": "Just me"}
	if functions.Truthy(viewer["isAdmin"]) {
		return []map[string]interface{}{
			{"value": "user", "name": "A specific user"},
			{"value": "group", "name": "A group"},
			{"value": "global", "name": "Everyone"},
		}
	}
	role, _ := viewer["role"].(string)
	if role == "group_admin" {
		return []map[string]interface{}{
			self,
			{"value": "group", "name": "My whole group"},
			{"value": "user", "name": "A specific user in my group"},
		}
	}
	return []map[string]interface{}{self}
}

// targetGroupOptions lists the groups a viewer may pick for scope="group": any
// group for an admin, only their own (access-level) group for group_admin,
// none for anyone else.
func targetGroupOptions(ctx context.Context, viewer map[string]interface{}) []map[string]interface{} {
	isAdmin := functions.Truthy(viewer["isAdmin"])
	role, _ := viewer["role"].(string)
	if !isAdmin && role != "group_admin" {
		return nil
	}
	db, err := pgdb.GetInstanceCtx(ctx)
	if err != nil {
		return nil
	}
	if isAdmin {
		rows, err := db.GetAll(`SELECT id AS value, name FROM user_groups ORDER BY name`)
		if err != nil {
			return nil
		}
		return toOptions(rows)
	}
	level := functions.Int(viewer["level"])
	rows, err := db.GetAll(`SELECT id AS value, name FROM user_groups WHERE id = $1`, level)
	if err != nil {
		return nil
	}
	return toOptions(rows)
}

func toOptions(rows []map[string]interface{}) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]interface{}{"value": r["value"], "name": functions.Coerce[string](r["name"])})
	}
	return out
}

// targetUserSearch backs target_user_id's autocomplete: any user for an
// admin, only members of the caller's own group for group_admin, nobody for
// anyone else. The field is hidden from the latter at the schema level, but
// the autocomplete endpoint has no field-visibility gate of its own, so the
// check is repeated here.
func targetUserSearch(ctx context.Context, input string, _ map[string]interface{}) []AutoOption {
	s := cache.SessionFromContext(ctx)
	if s == nil {
		return []AutoOption{}
	}
	db, err := pgdb.GetInstanceCtx(ctx)
	if err != nil {
		return []AutoOption{}
	}
	var rows []map[string]interface{}
	switch {
	case s.IsAdmin:
		rows, err = db.GetAll(
			`SELECT id, user_name AS name FROM users WHERE user_name ILIKE $1 OR CAST(id AS TEXT) LIKE $1 ORDER BY user_name LIMIT 20`,
			"%"+input+"%",
		)
	case s.Role == "group_admin":
		rows, err = db.GetAll(
			`SELECT id, user_name AS name FROM users
			 WHERE (user_name ILIKE $1 OR CAST(id AS TEXT) LIKE $1) AND groups @> to_jsonb($2::int)
			 ORDER BY user_name LIMIT 20`,
			"%"+input+"%", s.AccessLevel,
		)
	default:
		return []AutoOption{}
	}
	if err != nil {
		return []AutoOption{}
	}
	out := make([]AutoOption, 0, len(rows))
	for _, r := range rows {
		out = append(out, AutoOption{Value: functions.Coerce[string](r["id"]), Label: functions.Coerce[string](r["name"])})
	}
	return out
}

// enforceScopeAuthority is the server-side counterpart to scopeOptions/
// targetGroupOptions/targetUserSearch: those only shape what the UI offers,
// so a crafted request could submit any scope/target regardless. This clamps
// back to "self" whenever the submitter isn't actually authorized for the
// scope (and, for group_admin, the specific group/user) they asked for.
func enforceScopeAuthority(r *http.Request, data map[string]interface{}) {
	scope, _ := data["scope"].(string)
	if scope == "" || scope == "self" {
		return
	}
	s := cache.SessionFromContext(r.Context())
	if s == nil {
		data["scope"] = "self"
		return
	}
	if s.IsAdmin {
		return
	}
	if s.Role != "group_admin" || (scope != "group" && scope != "user") {
		data["scope"] = "self"
		return
	}
	if scope == "group" {
		if functions.Int(data["target_group_id"]) != s.AccessLevel {
			data["target_group_id"] = s.AccessLevel
		}
		return
	}
	// scope == "user": the target must be a member of the group_admin's own group.
	uid := functions.Int(data["target_user_id"])
	db, err := pgdb.GetInstanceCtx(r.Context())
	if err != nil {
		data["scope"] = "self"
		return
	}
	row, err := db.GetOne(`SELECT 1 AS x FROM users WHERE id = $1 AND groups @> to_jsonb($2::int)`, uid, s.AccessLevel)
	if err != nil || row == nil {
		data["scope"] = "self"
	}
}

func targetUserLabel(ctx context.Context, id string, _ map[string]interface{}) (string, bool) {
	db, err := pgdb.GetInstanceCtx(ctx)
	if err != nil {
		return "", false
	}
	row, err := db.GetOne("SELECT user_name AS name FROM users WHERE id = $1", functions.Int(id))
	if err != nil || row == nil {
		return "", false
	}
	name := functions.Coerce[string](row["name"])
	return name, name != ""
}

func fieldset() []Field {
	return []Field{
		NewField("title", TYPE_STRING, true).
			WithLabel("Title").
			WithValidation("maxLength", 200),
		NewField("description", TYPE_TEXT, false).
			WithLabel("Description"),

		// Who sees this event, resolved from the creator's own role — see
		// scopeOptions. Stored for display; visible_users/visible_groups
		// (below) are what the engine's row-visibility check actually reads.
		NewField("scope", TYPE_SELECT, true).
			WithLabel("Visible to").
			WithDefault("self").
			WithOptionsCtx(func(ctx context.Context, viewer map[string]interface{}) []map[string]interface{} {
				return scopeOptions(viewer)
			}),
		NewField("target_group_id", TYPE_INT, false).
			WithLabel("Group").
			WithDescription(`Only used when "Visible to" is a group`).
			WithOption("widget", "select").
			WithOptionsCtx(targetGroupOptions),
		NewField("target_user_id", TYPE_INT, false).
			WithLabel("User").
			WithDescription(`Only used when "Visible to" is a specific user (admin only)`).
			WithAutocomplete(map[string]interface{}{
				"function": targetUserSearch,
				"view":     targetUserLabel,
			}),

		NewField("starts_at", TYPE_DATE_TIME, true).
			WithLabel("Starts"),
		NewField("duration_minutes", TYPE_INT, true).
			WithLabel("Duration (minutes)").
			WithDefault(60).
			WithValidation("min", 1),

		NewField("notify", TYPE_CHECKBOX, false).
			WithLabel("Remind me").
			WithDefault(false),
		NewField("notify_offset_minutes", TYPE_SELECT, false).
			WithLabel("Reminder").
			WithDefault(10).
			WithOptions(notifyOffsetOptions),

		NewField("repeat", TYPE_SELECT, false).
			WithLabel("Repeat").
			WithDefault("none").
			WithOptions(repeatOptions),

		// Push-only sync (see go/engine/pages/profile/googlepush.go, called from
		// AfterFieldset below): checking this pushes the event to the user's
		// connected Google Calendar on save. No pulling — this app never stores
		// an event it didn't create. A no-op if the user has no connected
		// account; always shown regardless (same "present but contextually
		// inert" convention as scope/target_group_id above — there's no
		// mechanism to hide a checkbox per-viewer the way a select's options
		// can be narrowed).
		NewField("sync_with_google", TYPE_CHECKBOX, false).
			WithLabel("Sync with Google Calendar").
			WithDefault(true),

		// Derived sharing lists the row-visibility engine reads (see
		// VisibilityUsersField/VisibilityGroupsField below) — computed from
		// scope/target_group_id/target_user_id by AfterFieldset, never
		// submitted by the client directly. Never shown in any UI mode.
		NewField("visible_users", TYPE_JSON, false).AsReadOnly().InModes(0),
		NewField("visible_groups", TYPE_JSON, false).AsReadOnly().InModes(0),

		// Notification-hub bookkeeping (Phase 4) — when the reminder last fired,
		// so it's sent exactly once. Never shown in any UI mode.
		NewField("notified_at", TYPE_DATE_TIME, false).AsReadOnly().InModes(0),

		// The id of this event's pushed copy on the user's Google Calendar, once
		// sync_with_google has pushed it at least once — lets AfterFieldset
		// update that same Google event in place on later edits instead of
		// creating a duplicate. Never shown in any UI mode.
		NewField("google_event_id", TYPE_STRING, false).AsReadOnly().InModes(0),
	}
}

func filters() *ListFilters {
	return NewFieldset(
		NewFilter("from", TYPE_DATE_TIME).WithSQL("starts_at").GreaterOrEqual(),
		NewFilter("to", TYPE_DATE_TIME).WithSQL("starts_at").LessOrEqual(),
	)
}

// deriveVisibility folds scope/target_group_id/target_user_id into the jsonb
// sharing lists buildVisibilityCondition reads. scope="self" needs neither —
// the engine already grants the creator visibility via created_by. scope=
// "global" needs neither either — it's matched by VisibilityExtraWhere below.
func deriveVisibility(data map[string]interface{}) {
	scope, _ := data["scope"].(string)
	switch scope {
	case "group":
		if gid := functions.Int(data["target_group_id"]); gid > 0 {
			data["visible_groups"] = []int{gid}
		}
	case "user":
		if uid := functions.Int(data["target_user_id"]); uid > 0 {
			data["visible_users"] = []int{uid}
		}
	}
}

// pushToGoogleIfNeeded pushes the event to the creator's connected Google
// Calendar when sync_with_google is checked (updating its existing Google
// copy in place on an edit, via google_event_id), or deletes that copy if
// the box was just unchecked. filterValidFields only includes fields the
// client actually submitted, so an edit that only touches e.g. the title
// would otherwise push an incomplete event — the existing row's stored
// values are read first and `data` overlaid on top of them.
//
// Never fails the save: a push error (no connected account, Google API
// error, token revoked) is swallowed — the event always saves locally
// regardless of Google's availability.
func pushToGoogleIfNeeded(r *http.Request, data map[string]interface{}) {
	s := cache.SessionFromContext(r.Context())
	if s == nil || s.UserID <= 0 {
		return
	}

	db, err := pgdb.GetInstanceCtx(r.Context())
	if err != nil {
		return
	}

	merged := map[string]interface{}{}
	if id := mux.Vars(r)["id"]; id != "" {
		if row, e := db.GetOne(
			"SELECT title, description, starts_at, duration_minutes, repeat, sync_with_google, google_event_id FROM events WHERE id = $1",
			id,
		); e == nil && row != nil {
			for k, v := range row {
				merged[k] = v
			}
		}
	}
	for k, v := range data {
		merged[k] = v
	}

	syncOn := functions.Truthy(merged["sync_with_google"])
	googleEventID := functions.Coerce[string](merged["google_event_id"])

	if !syncOn {
		if googleEventID != "" {
			_ = profile.DeleteGoogleEvent(r.Context(), s.UserID, googleEventID)
			data["google_event_id"] = ""
		}
		return
	}

	start := functions.Coerce[time.Time](merged["starts_at"])
	if start.IsZero() {
		return
	}

	newID, err := profile.PushEventToGoogle(r.Context(), s.UserID, profile.GoogleEventPush{
		GoogleEventID:   googleEventID,
		Title:           functions.Coerce[string](merged["title"]),
		Description:     functions.Coerce[string](merged["description"]),
		StartsAt:        start,
		DurationMinutes: functions.Int(merged["duration_minutes"]),
		Repeat:          functions.Coerce[string](merged["repeat"]),
	})
	if err == nil && newID != "" {
		data["google_event_id"] = newID
	}
}

var Module = &ModuleAbstract[interface{}]{
	ID:     "events",
	Name:   "Events",
	Icon:   "calendar",
	Rights: make(map[int]int),
	// Everyone may list/view their own scope's events; creating/editing is
	// gated the same way (any authenticated user may create a "self" event).
	DefaultPermission:    1, // PERMISSION_READ
	DefaultPermissionSet: true,
	// Sharing-list visibility: a non-admin sees an event only if they created
	// it, or it's shared with them via visible_users/visible_groups, or it's
	// global (VisibilityExtraWhere) — see fieldset_engine.buildVisibilityCondition.
	VisibilityUsersField:  "visible_users",
	VisibilityGroupsField: "visible_groups",
	VisibilityExtraWhere:  "scope = 'global'",
	CustomRoutes: []CustomRoute{
		{Path: "/api/events/due-count", Methods: []string{"GET"}, Handler: handleDueCount, Absolute: true},
	},
}

func init() {
	Module.Fields = fieldset()
	Module.Filters = filters()
	Module.AfterFieldset = func(r *http.Request, data map[string]interface{}) (map[string]interface{}, error) {
		enforceScopeAuthority(r, data)
		deriveVisibility(data)
		pushToGoogleIfNeeded(r, data)
		return data, nil
	}
	app.RegisterModule(Module, "events")
}
