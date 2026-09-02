package auth

import (
	"encoding/json"
	"strings"

	"tls-rest/go/engine/controllers/db/pgdb"
	"tls-rest/go/engine/controllers/functions"
)

// This file is the single live bridge between the database and the per-mode
// bitmask access model declared in access.go.
//
// Model (per the revised rights design):
//   - Every user belongs to at most one group, referenced by users.user_group.
//   - Groups are ordered by id: 0 (or no group) = unauthorized ... n = higher
//     privilege. A group's id IS the user's access level.
//   - A group may be flagged is_admin; admins bypass mode and row checks.
//   - Rights are per-mode bitmasks (auth.MODE_*) split across two tables:
//       user_group_rights(group_id, module, modes)  - rights for a whole group
//       user_rights(user_id,  module, modes)         - rights for one user
//     A user's effective modes for a module are the OR of the module default,
//     their group's rights, and their own rights (user rights are additive).

// userGroupsSubquery yields every group id a user belongs to: the primary group
// on the user row (users.user_group) UNION any additional memberships in
// user_group_members. Callers reference $1 = userID (used in both halves). This
// is how a user can belong to multiple groups; rights are aggregated across all.
const userGroupsSubquery = `
	SELECT user_group AS group_id FROM users WHERE id = $1 AND user_group IS NOT NULL
	UNION
	SELECT group_id FROM user_group_members WHERE user_id = $1
`

// defaultModesFor maps a module's registered default permission
// (DENY/READ/WRITE) to a mode bitmask, so a module is usable before any
// explicit rights are granted: DENY -> nothing, READ -> browse/read,
// WRITE -> everything.
func defaultModesFor(perm int) int {
	switch {
	case perm >= PERMISSION_WRITE:
		return MODE_ALL
	case perm >= PERMISSION_READ:
		return MODE_LIST | MODE_VIEW
	default:
		return 0
	}
}

// ResolveModuleFieldRights builds the per-module allowed-field set for a user.
// A module ABSENT from the result is unrestricted (all fields — current
// behavior); a module PRESENT maps to the exact set of non-system fields the
// user may access. Rights are additive: an empty `fields` value on ANY
// applicable rights row means "all fields" and leaves the module unrestricted;
// otherwise the allowed set is the union of listed fields across the user's
// group rights and their own user rights.
//
// If the `fields` column doesn't exist yet (rights tables predating this
// feature), the queries error and the result is empty — i.e. unrestricted,
// preserving existing behavior until the column is added.
func ResolveModuleFieldRights(userID int) map[string]map[string]int {
	db, err := pgdb.GetInstance()
	if err != nil {
		return map[string]map[string]int{}
	}

	// module -> field -> allowed-mode bitmask (union across the user's rows).
	acc := map[string]map[string]int{}
	unrestricted := map[string]bool{} // module -> saw an "all fields" (empty) row

	consume := func(rows []map[string]interface{}) {
		for _, row := range rows {
			m, _ := row["module"].(string)
			if m == "" {
				continue
			}
			perField, empty := fieldRightsFromValue(row["fields"])
			if empty || len(perField) == 0 {
				unrestricted[m] = true // empty / unparseable = all fields
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

	if rows, e := db.RQuery(`
		SELECT ugr.module AS module, ugr.fields AS fields
		FROM user_group_rights ugr
		WHERE ugr.group_id IN (`+userGroupsSubquery+`)
	`, userID); e == nil {
		consume(rows)
	}
	if rows, e := db.RQuery(`
		SELECT module, fields FROM user_rights WHERE user_id = $1
	`, userID); e == nil {
		consume(rows)
	}

	result := map[string]map[string]int{}
	for m, fields := range acc {
		if !unrestricted[m] { // an "all fields" row anywhere wins
			result[m] = fields
		}
	}
	return result
}

// fieldRightsFromValue normalizes a stored "fields" value — which may arrive as
// a string/[]byte (TEXT column) or an already-parsed map (JSONB column) — into
// field -> mode-bitmask. The second return is true when the value is empty
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

// ResolveModuleModeRights builds the per-module allowed-mode bitmask for a user.
// It seeds every registered module with its default modes, then OR-s in the
// user's group rights and their own user rights. Anonymous users (id <= 0)
// receive only the defaults, so public modules stay readable while restricted
// modules stay closed.
func ResolveModuleModeRights(userID int) ModuleModeRights {
	rights := ModuleModeRights{}
	for module, def := range ModuleDefaults() {
		rights[module] = defaultModesFor(def)
	}

	if userID <= 0 {
		return rights
	}

	db, err := pgdb.GetInstance()
	if err != nil {
		return rights
	}

	// Group rights, OR-ed across every group the user belongs to (the primary
	// group on the user row plus any additional memberships).
	if rows, gerr := db.RQuery(`
		SELECT ugr.module AS module, ugr.modes AS modes
		FROM user_group_rights ugr
		WHERE ugr.group_id IN (`+userGroupsSubquery+`)
	`, userID); gerr == nil {
		for _, row := range rows {
			if m, _ := row["module"].(string); m != "" {
				rights[m] |= functions.Int(row["modes"])
			}
		}
	}

	// User-specific rights, additive on top of the group's.
	if rows, uerr := db.RQuery(`
		SELECT module, modes FROM user_rights WHERE user_id = $1
	`, userID); uerr == nil {
		for _, row := range rows {
			if m, _ := row["module"].(string); m != "" {
				rights[m] |= functions.Int(row["modes"])
			}
		}
	}

	return rights
}

// ResolveUserAccessLevel returns the user's access level, which is the id of the
// group they belong to (0 / AccessAll if they have none). A record is visible
// when the user's level is >= the record's access level (see CanAccessRow);
// admins bypass the check entirely.
func ResolveUserAccessLevel(userID int) int {
	if userID <= 0 {
		return AccessAll
	}

	db, err := pgdb.GetInstance()
	if err != nil {
		return AccessAll
	}

	rows, err := db.RQuery(`
		SELECT COALESCE(MAX(group_id), 0) AS level
		FROM (`+userGroupsSubquery+`) g
	`, userID)
	if err != nil || len(rows) == 0 {
		return AccessAll
	}

	return functions.Int(rows[0]["level"])
}

// ResolveIsAdmin reports whether the user's group is flagged as an administrator
// group. Admins bypass every mode and row-access check.
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
