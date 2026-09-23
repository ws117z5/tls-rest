package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	config "tls-rest/go/app/constants"
	"tls-rest/go/engine/controllers/db/pgdb"
	"tls-rest/go/engine/controllers/functions"
)

const userGroupsSubquery = `
	SELECT jsonb_array_elements_text(groups)::int AS group_id FROM users WHERE id = $1
`

// GuestGroupID re-exports constants.GuestGroupID (module can't import auth,
// so auth re-exports it) — what an anonymous caller resolves as.
var GuestGroupID = config.GuestGroupID

// AdminGroupID re-exports constants.AdminGroupID.
var AdminGroupID = config.AdminGroupID

// UsersGroupID re-exports constants.UsersGroupID.
var UsersGroupID = config.UsersGroupID

// groupIDsExpr returns the SQL (aliased "group_id") and args yielding the
// group ids to resolve rights for: a real user's own groups, or just
// GuestGroupID when anonymous (userID <= 0).
func groupIDsExpr(userID int) (expr string, args []interface{}) {
	if userID <= 0 {
		return fmt.Sprintf("SELECT %d AS group_id", GuestGroupID), nil
	}
	return userGroupsSubquery, []interface{}{userID}
}

// defaultModesFor maps a module's default permission to a mode bitmask:
// DENY -> nothing, READ -> browse/read, WRITE -> everything. MODE_FILTERS is
// never part of a default — it's always an explicit grant (see canFilter).
func defaultModesFor(perm int) int {
	switch {
	case perm >= PERMISSION_WRITE:
		return MODE_ALL &^ MODE_FILTERS
	case perm >= PERMISSION_READ:
		return MODE_LIST | MODE_VIEW
	default:
		return 0
	}
}

// ResolveModuleFieldRights builds the per-module allowed-field set for a
// user: a module absent from the result is unrestricted; an empty `fields`
// value on any applicable row also leaves it unrestricted (rights are additive).
func ResolveModuleFieldRights(userID int) map[string]map[string]int {
	db, err := pgdb.GetInstance()
	if err != nil {
		return map[string]map[string]int{}
	}

	acc := map[string]map[string]int{}
	unrestricted := map[string]bool{}

	consume := func(rows []map[string]interface{}) {
		for _, row := range rows {
			m, _ := row["module"].(string)
			if m == "" {
				continue
			}
			perField, empty := fieldRightsFromValue(row["fields"])
			if empty || len(perField) == 0 {
				unrestricted[m] = true
				continue
			}
			if acc[m] == nil {
				acc[m] = map[string]int{}
			}
			for field, mask := range perField {
				acc[m][field] |= mask
			}
		}
	}

	groupExpr, groupArgs := groupIDsExpr(userID)
	if rows, e := db.RQuery(`
		SELECT ugr.module AS module, ugr.fields AS fields
		FROM user_group_rights ugr
		WHERE ugr.group_id IN (`+groupExpr+`)
	`, groupArgs...); e == nil {
		consume(rows)
	}
	if userID > 0 {
		if rows, e := db.RQuery(`
			SELECT module, fields FROM user_rights WHERE user_id = $1
		`, userID); e == nil {
			consume(rows)
		}
	}

	result := map[string]map[string]int{}
	for m, fields := range acc {
		if !unrestricted[m] {
			result[m] = fields
		}
	}
	return result
}

// ResolveModuleFilterFieldRights is ResolveModuleFieldRights' counterpart for
// filters.go's own fieldset (see filterFieldsFromValue / user_group_rights,
// user_rights.filter_fields): a module absent from the result is unrestricted.
func ResolveModuleFilterFieldRights(ctx context.Context, userID int) map[string]map[string]bool {
	db, err := pgdb.GetInstanceCtx(ctx)
	if err != nil {
		return map[string]map[string]bool{}
	}

	acc := map[string]map[string]bool{}
	unrestricted := map[string]bool{}
	consume := func(rows []map[string]interface{}) {
		for _, row := range rows {
			m, _ := row["module"].(string)
			if m == "" {
				continue
			}
			allowed, empty := filterFieldsFromValue(row["filter_fields"])
			if empty || len(allowed) == 0 {
				unrestricted[m] = true
				continue
			}
			if acc[m] == nil {
				acc[m] = map[string]bool{}
			}
			for name := range allowed {
				acc[m][name] = true
			}
		}
	}

	groupExpr, groupArgs := groupIDsExpr(userID)
	if rows, e := db.RQuery(`
		SELECT ugr.module AS module, ugr.filter_fields AS filter_fields
		FROM user_group_rights ugr
		WHERE ugr.group_id IN (`+groupExpr+`)
	`, groupArgs...); e == nil {
		consume(rows)
	}
	if userID > 0 {
		if rows, e := db.RQuery(`
			SELECT module, filter_fields FROM user_rights WHERE user_id = $1
		`, userID); e == nil {
			consume(rows)
		}
	}

	result := map[string]map[string]bool{}
	for m, names := range acc {
		if !unrestricted[m] {
			result[m] = names
		}
	}
	return result
}

// fieldRightsFromValue normalizes a stored "fields" value (string, []byte, or
// parsed map) into field -> mode-bitmask; the bool is true when empty
// (meaning "all fields", unrestricted).
func fieldRightsFromValue(v interface{}) (map[string]int, bool) {
	switch t := v.(type) {
	case nil:
		return nil, true
	case string:
		if strings.TrimSpace(t) == "" {
			return nil, true
		}
		return parseFieldRights(t), false
	case []byte:
		if strings.TrimSpace(string(t)) == "" {
			return nil, true
		}
		return parseFieldRights(string(t)), false
	case map[string]interface{}:
		if len(t) == 0 {
			return nil, true
		}
		out := map[string]int{}
		for field, modes := range t {
			mask := 0
			if arr, ok := modes.([]interface{}); ok {
				for _, name := range arr {
					if s, ok := name.(string); ok {
						mask |= modeBit(s)
					}
				}
			}
			out[field] = mask
		}
		return out, false
	default:
		return nil, true
	}
}

// parseFieldRights parses a stored "fields" value into field -> mode-bitmask.
// Accepts the JSON table format {"title":["view","edit"]} and, for backward
// compatibility, a legacy CSV of field names (each granted all modes).
func parseFieldRights(raw string) map[string]int {
	out := map[string]int{}
	if strings.HasPrefix(strings.TrimSpace(raw), "{") {
		var m map[string][]string
		if err := json.Unmarshal([]byte(raw), &m); err == nil {
			for field, modes := range m {
				mask := 0
				for _, name := range modes {
					mask |= modeBit(name)
				}
				out[field] = mask
			}
			return out
		}
	}
	for _, f := range strings.Split(raw, ",") {
		if f = strings.TrimSpace(f); f != "" {
			out[f] = allModesMask()
		}
	}
	return out
}

// modeBit maps a mode name to its bit using the canonical table in access.go.
func modeBit(name string) int {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, m := range modeNameTable {
		if m.name == name {
			return int(m.bit)
		}
	}
	return 0
}

// allModesMask is the OR of every named mode bit.
func allModesMask() int {
	mask := 0
	for _, m := range modeNameTable {
		mask |= int(m.bit)
	}
	return mask
}

// resolveSessionRights is fillSessionRights's single-query path: module mode
// rights, per-module field/filter rights, access level and admin status in
// one round trip, instead of calling the Resolve* functions below separately.
// Those stay as-is for their other, narrower callers.
func resolveSessionRights(ctx context.Context, userID int) (modes ModuleModeRights, fieldRights map[string]map[string]int, specialRights map[string]map[string]bool, filterFieldRights map[string]map[string]bool, accessLevel int, isAdmin bool) {
	modes = ModuleModeRights{}
	for module, def := range ModuleDefaults() {
		modes[module] = defaultModesFor(def)
	}
	for page, m := range PageDefaults() {
		modes[page] = m
	}
	accessLevel = AccessAll
	fieldRights = map[string]map[string]int{}
	specialRights = map[string]map[string]bool{}
	filterFieldRights = map[string]map[string]bool{}

	db, err := pgdb.GetInstanceCtx(ctx)
	if err != nil {
		return modes, fieldRights, specialRights, filterFieldRights, accessLevel, false
	}

	query, args := combinedRightsQuery(userID)
	rows, err := db.RQuery(query, args...)
	if err != nil || len(rows) == 0 {
		return modes, fieldRights, specialRights, filterFieldRights, accessLevel, false
	}
	row := rows[0]

	accessLevel = functions.Int(row["access_level"])
	if userID > 0 {
		isAdmin, _ = row["is_admin"].(bool)
	}

	acc := map[string]map[string]int{}
	unrestricted := map[string]bool{}
	filterAcc := map[string]map[string]bool{}
	filterUnrestricted := map[string]bool{}
	applyRights := func(list interface{}) {
		items, _ := list.([]interface{})
		for _, item := range items {
			entry, _ := item.(map[string]interface{})
			if entry == nil {
				continue
			}
			m, _ := entry["module"].(string)
			if m == "" {
				continue
			}
			modes[m] |= functions.Int(entry["modes"])

			perField, empty := fieldRightsFromValue(entry["fields"])
			if empty || len(perField) == 0 {
				unrestricted[m] = true
			} else {
				if acc[m] == nil {
					acc[m] = map[string]int{}
				}
				for field, mask := range perField {
					acc[m][field] |= mask
				}
			}

			allowedFilters, filtersEmpty := filterFieldsFromValue(entry["filter_fields"])
			if filtersEmpty || len(allowedFilters) == 0 {
				filterUnrestricted[m] = true
			} else {
				if filterAcc[m] == nil {
					filterAcc[m] = map[string]bool{}
				}
				for name := range allowedFilters {
					filterAcc[m][name] = true
				}
			}

			for _, id := range specialRightIDs(entry["special_rights"]) {
				if specialRights[m] == nil {
					specialRights[m] = map[string]bool{}
				}
				specialRights[m][id] = true
			}
		}
	}
	applyRights(row["group_rights"])
	applyRights(row["user_rights"])

	for m, names := range filterAcc {
		if !filterUnrestricted[m] {
			filterFieldRights[m] = names
		}
	}

	for m, fields := range acc {
		if !unrestricted[m] {
			fieldRights[m] = fields
		}
	}

	return modes, fieldRights, specialRights, filterFieldRights, accessLevel, isAdmin
}

// filterFieldsFromValue is fieldRightsFromValue's counterpart for a stored
// `filter_fields` value — a JSON array of allowed filter names, not a
// per-field mode map, since a filter is only ever usable or not.
func filterFieldsFromValue(v interface{}) (map[string]bool, bool) {
	var raw []byte
	switch t := v.(type) {
	case nil:
		return nil, true
	case string:
		raw = []byte(t)
	case []byte:
		raw = t
	default:
		return nil, true
	}
	if strings.TrimSpace(string(raw)) == "" {
		return nil, true
	}
	var names []string
	if err := json.Unmarshal(raw, &names); err != nil || len(names) == 0 {
		return nil, true
	}
	out := make(map[string]bool, len(names))
	for _, n := range names {
		out[n] = true
	}
	return out, false
}

// specialRightIDs parses a stored `special_rights` value (a JSON string/[]byte
// array of right ids) into a slice; unparseable yields none.
func specialRightIDs(v interface{}) []string {
	var raw []byte
	switch t := v.(type) {
	case string:
		raw = []byte(t)
	case []byte:
		raw = t
	default:
		return nil
	}
	var out []string
	_ = json.Unmarshal(raw, &out)
	return out
}

// combinedRightsQuery builds resolveSessionRights's single query: the
// caller's group ids (GuestGroupID when anonymous), their module mode/field
// rights from both user_group_rights and user_rights, access level (highest
// group id), and admin status — all as one row via CTEs and json_agg.
func combinedRightsQuery(userID int) (string, []interface{}) {
	var args []interface{}

	groupsCTE := fmt.Sprintf("SELECT %d AS group_id", GuestGroupID)
	userRightsFilter := "NULL"
	if userID > 0 {
		args = append(args, userID)
		groupsCTE = fmt.Sprintf("SELECT jsonb_array_elements_text(groups)::int AS group_id FROM users WHERE id = $%d", len(args))

		args = append(args, userID)
		userRightsFilter = fmt.Sprintf("$%d", len(args))
	}

	query := fmt.Sprintf(`
		WITH groups AS (%s),
		gr AS (
			SELECT module, modes, fields, special_rights, filter_fields FROM user_group_rights WHERE group_id IN (SELECT group_id FROM groups)
		),
		ur AS (
			SELECT module, modes, fields, special_rights, filter_fields FROM user_rights WHERE user_id = %s
		)
		SELECT
			COALESCE(MAX(group_id), 0) AS access_level,
			EXISTS (SELECT 1 FROM user_groups ug WHERE ug.is_admin AND ug.id IN (SELECT group_id FROM groups)) AS is_admin,
			COALESCE((SELECT jsonb_agg(jsonb_build_object('module', module, 'modes', modes, 'fields', fields, 'special_rights', special_rights, 'filter_fields', filter_fields)) FROM gr), '[]'::jsonb) AS group_rights,
			COALESCE((SELECT jsonb_agg(jsonb_build_object('module', module, 'modes', modes, 'fields', fields, 'special_rights', special_rights, 'filter_fields', filter_fields)) FROM ur), '[]'::jsonb) AS user_rights
		FROM groups
	`, groupsCTE, userRightsFilter)

	return query, args
}

// ResolveModuleModeRights builds the per-module allowed-mode bitmask for a
// user: module default, OR-ed with group rights, OR-ed with the user's own
// rights. An anonymous caller (id <= 0) resolves as a member of GuestGroupID.
func ResolveModuleModeRights(ctx context.Context, userID int) ModuleModeRights {
	rights := ModuleModeRights{}
	for module, def := range ModuleDefaults() {
		rights[module] = defaultModesFor(def)
	}
	for page, modes := range PageDefaults() {
		rights[page] = modes
	}

	db, err := pgdb.GetInstanceCtx(ctx)
	if err != nil {
		return rights
	}

	groupExpr, groupArgs := groupIDsExpr(userID)
	if rows, gerr := db.RQuery(`
		SELECT ugr.module AS module, ugr.modes AS modes
		FROM user_group_rights ugr
		WHERE ugr.group_id IN (`+groupExpr+`)
	`, groupArgs...); gerr == nil {
		for _, row := range rows {
			if m, _ := row["module"].(string); m != "" {
				rights[m] |= functions.Int(row["modes"])
			}
		}
	}

	if userID > 0 {
		if rows, uerr := db.RQuery(`
			SELECT module, modes FROM user_rights WHERE user_id = $1
		`, userID); uerr == nil {
			for _, row := range rows {
				if m, _ := row["module"].(string); m != "" {
					rights[m] |= functions.Int(row["modes"])
				}
			}
		}
	}

	return rights
}

// ResolveUserAccessLevel returns the id of the highest group the caller
// belongs to (0 / AccessAll if none); an anonymous caller (id <= 0) resolves
// as a member of GuestGroupID, same as ResolveModuleModeRights.
func ResolveUserAccessLevel(userID int) int {
	db, err := pgdb.GetInstance()
	if err != nil {
		return AccessAll
	}

	groupExpr, groupArgs := groupIDsExpr(userID)
	rows, err := db.RQuery(`
		SELECT COALESCE(MAX(group_id), 0) AS level
		FROM (`+groupExpr+`) g
	`, groupArgs...)
	if err != nil || len(rows) == 0 {
		return AccessAll
	}

	return functions.Int(rows[0]["level"])
}

// ResolveIsAdmin reports whether the user's group is flagged as an
// administrator group. Unlike the other Resolve* functions, an anonymous
// caller is never resolved via GuestGroupID here — a guest is never admin.
func ResolveIsAdmin(userID int) bool {
	if userID <= 0 {
		return false
	}

	db, err := pgdb.GetInstance()
	if err != nil {
		return false
	}

	rows, err := db.RQuery(`
		SELECT EXISTS (
			SELECT 1 FROM user_groups ug
			WHERE ug.is_admin AND ug.id IN (`+userGroupsSubquery+`)
		) AS is_admin
	`, userID)
	if err != nil || len(rows) == 0 {
		return false
	}

	b, _ := rows[0]["is_admin"].(bool)
	return b
}
