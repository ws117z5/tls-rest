package auth

import (
	"tls-rest/go/engine/controllers/db/pgdb"
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

// toInt best-effort converts a value scanned by go-pg (int64 for integer
// columns, but also int/float64/nil in edge cases) into an int.
func toInt(v interface{}) int {
	switch n := v.(type) {
	case int64:
		return int(n)
	case int:
		return n
	case int32:
		return int(n)
	case float64:
		return int(n)
	case nil:
		return 0
	default:
		return 0
	}
}

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

	// Group rights, via the single group referenced on the user row.
	if rows, gerr := db.RQuery(`
		SELECT ugr.module AS module, ugr.modes AS modes
		FROM users u
		JOIN user_group_rights ugr ON ugr.group_id = u.user_group
		WHERE u.id = $1
	`, userID); gerr == nil {
		for _, row := range rows {
			if m, _ := row["module"].(string); m != "" {
				rights[m] |= toInt(row["modes"])
			}
		}
	}

	// User-specific rights, additive on top of the group's.
	if rows, uerr := db.RQuery(`
		SELECT module, modes FROM user_rights WHERE user_id = $1
	`, userID); uerr == nil {
		for _, row := range rows {
			if m, _ := row["module"].(string); m != "" {
				rights[m] |= toInt(row["modes"])
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
		SELECT COALESCE(user_group, 0) AS level
		FROM users
		WHERE id = $1
	`, userID)
	if err != nil || len(rows) == 0 {
		return AccessAll
	}

	return toInt(rows[0]["level"])
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
		SELECT ug.is_admin AS is_admin
		FROM users u
		JOIN user_groups ug ON ug.id = u.user_group
		WHERE u.id = $1
	`, userID)
	if err != nil || len(rows) == 0 {
		return false
	}

	b, _ := rows[0]["is_admin"].(bool)
	return b
}
