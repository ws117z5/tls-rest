package auth

import (
	"encoding/json"
	"fmt"
	"strings"

	config "tls-rest/go/constants"
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
// DENY -> nothing, READ -> browse/read, WRITE -> everything.
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

// ResolveModuleModeRights builds the per-module allowed-mode bitmask for a
// user: module default, OR-ed with group rights, OR-ed with the user's own
// rights. An anonymous caller (id <= 0) resolves as a member of GuestGroupID.
func ResolveModuleModeRights(userID int) ModuleModeRights {
	rights := ModuleModeRights{}
	for module, def := range ModuleDefaults() {
		rights[module] = defaultModesFor(def)
	}

	db, err := pgdb.GetInstance()
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
