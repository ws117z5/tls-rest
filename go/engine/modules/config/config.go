// Package config is the admin CRUD module for configuration. Each row targets a
// scope (global/group/user) with an optional scope_id, and sets named parameters
// (theme, date_format). Effective config is resolved global < group < user by
// engine/controllers/config.
package config

import (
	"strconv"

	"tls-rest/go/engine/controllers/db/pgdb"
	. "tls-rest/go/engine/controllers/field"
	"tls-rest/go/engine/controllers/functions"
	. "tls-rest/go/engine/controllers/module"
)

var Module = &ModuleAbstract[interface{}]{
	ID:              "config",
	Name:            "Config",
	Submenu:         "engine",
	ConfigAffecting: true, // writes invalidate cached session config
	Fields: []Field{
		NewField("scope", TYPE_SELECT, true).
			WithLabel("Scope").
			WithDescription("global, group, or user").
			WithDefault("global").
			WithOptions(func() []map[string]interface{} {
				return []map[string]interface{}{
					{"value": "global", "name": "Global"},
					{"value": "group", "name": "Group"},
					{"value": "user", "name": "User"},
				}
			}),
		NewField("scope_id", TYPE_INT, false).
			WithLabel("Scope ID").
			WithDescription("search a user or group (by the chosen scope); 0 for global").
			WithDefault(0).
			WithAutocomplete("function", func(input string, values map[string]interface{}) []AutoOption {
				scope, _ := values["scope"].(string)
				if scope == "global" || scope == "" {
					return []AutoOption{}
				}
				db, err := pgdb.GetInstance()
				if err != nil {
					return []AutoOption{}
				}
				var q string
				switch scope {
				case "user":
					q = `SELECT id, user_name AS name FROM users
					     WHERE user_name ILIKE $1 OR CAST(id AS TEXT) LIKE $1
					     ORDER BY user_name LIMIT 20`
				case "group":
					q = `SELECT id, name FROM user_groups
					     WHERE name ILIKE $1 OR CAST(id AS TEXT) LIKE $1
					     ORDER BY name LIMIT 20`
				default:
					return []AutoOption{}
				}
				rows, err := db.RQuery(q, "%"+input+"%")
				if err != nil {
					return []AutoOption{}
				}
				out := make([]AutoOption, 0, len(rows))
				for _, r := range rows {
					id := functions.Coerce[int](r["id"])
					name := functions.Coerce[string](r["name"])
					out = append(out, AutoOption{
						Value: strconv.Itoa(id),
						Label: name + " (#" + strconv.Itoa(id) + ")",
					})
				}
				return out
			}),

		// Parameters — one field each, not a generic key/value.
		NewField("theme", TYPE_SELECT, false).
			WithLabel("Theme").
			WithOptions(func() []map[string]interface{} {
				return []map[string]interface{}{
					{"value": "light", "name": "Light"},
					{"value": "dark", "name": "Dark"},
				}
			}),
		NewField("date_format", TYPE_STRING, false).
			WithLabel("Date format").
			WithDescription("e.g. YYYY-MM-DD, DD/MM/YYYY"),
	},
	// Administration module: no access unless explicitly granted (or admin).
	DefaultPermission:    PERMISSION_DENY,
	DefaultPermissionSet: true,
	Rights:               make(map[int]int),
}

func Init() { Module.Initialize("config") }
