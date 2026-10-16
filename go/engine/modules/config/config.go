// Package config is the admin CRUD module for configuration. Each row targets a
// scope (global/group/user) with an optional scope_id, and sets named parameters
// (theme, date_format). Effective config is resolved global < group < user by
// engine/controllers/config.
package config

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"

	"tls-rest/go/app"
	"tls-rest/go/engine/controllers/db/pgdb"
	. "tls-rest/go/engine/controllers/field"
	"tls-rest/go/engine/controllers/functions"
	. "tls-rest/go/engine/controllers/module"
)

// modeOptions lists the fieldset engine's own modes — the same set a module's CustomViews map is keyed by.
func modeOptions() []map[string]interface{} {
	return []map[string]interface{}{
		{"value": "list", "name": "List"},
		{"value": "view", "name": "View"},
		{"value": "edit", "name": "Edit"},
		{"value": "create", "name": "Create"},
	}
}

// moduleViewsTableData loads this config row's module_views column (a JSON array of {module,mode,view}) for the table field.
func moduleViewsTableData(ctx context.Context, data map[string]interface{}) []map[string]interface{} {
	id := functions.Int(data["id"])
	if id <= 0 {
		return nil
	}
	db, err := pgdb.GetInstanceCtx(ctx)
	if err != nil {
		return nil
	}
	row, err := db.GetOne("SELECT module_views FROM config WHERE id = $1", id)
	if err != nil || row == nil {
		return nil
	}
	return decodeModuleViewRows(row["module_views"])
}

// decodeModuleViewRows normalizes a config.module_views cell into row maps. pgx
// decodes a jsonb column into native Go values (here, []interface{} of
// map[string]interface{}) rather than handing back the raw JSON text, so a
// plain json.Unmarshal of it as a string (what this used to do) always failed
// and silently dropped every saved row. A string/[]byte is still accepted in
// case a caller ever hands this a pre-serialized value.
func decodeModuleViewRows(raw interface{}) []map[string]interface{} {
	switch v := raw.(type) {
	case []interface{}:
		out := make([]map[string]interface{}, 0, len(v))
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				out = append(out, m)
			}
		}
		return out
	case []map[string]interface{}:
		return v
	case string:
		if v == "" {
			return nil
		}
		var rows []map[string]interface{}
		if json.Unmarshal([]byte(v), &rows) != nil {
			return nil
		}
		return rows
	case []byte:
		if len(v) == 0 {
			return nil
		}
		var rows []map[string]interface{}
		if json.Unmarshal(v, &rows) != nil {
			return nil
		}
		return rows
	}
	return nil
}

// moduleViewOptions lists every view name that actually exists for a module_views row's own module+mode
// (read from that row's sibling cells): "default" and "system" (every module/mode has these), plus any
// CustomViews the module declares for that mode (module.go's CustomViews mode -> {viewName: label}).
func moduleViewOptions(_ context.Context, _ string, values map[string]interface{}) []AutoOption {
	moduleID, _ := values["module"].(string)
	mode, _ := values["mode"].(string)
	names := map[string]bool{"default": true, "system": true}
	if mod, ok := RegisteredModules[moduleID]; ok {
		for name := range mod.GetCustomViews()[mode] {
			names[name] = true
		}
	}
	sorted := make([]string, 0, len(names))
	for name := range names {
		sorted = append(sorted, name)
	}
	sort.Strings(sorted)
	out := make([]AutoOption, 0, len(sorted))
	for _, name := range sorted {
		out = append(out, AutoOption{Value: name, Label: name})
	}
	return out
}

// moduleViewsOnSubmit keeps only complete rows before they're stored as this row's module_views column.
func moduleViewsOnSubmit(rows []map[string]interface{}) interface{} {
	out := make([]map[string]interface{}, 0, len(rows))
	for _, r := range rows {
		module, _ := r["module"].(string)
		mode, _ := r["mode"].(string)
		view, _ := r["view"].(string)
		if module == "" || mode == "" || view == "" {
			continue
		}
		out = append(out, map[string]interface{}{"module": module, "mode": mode, "view": view})
	}
	return out
}

var Module = &ModuleAbstract[interface{}]{
	ID:              "config",
	Name:            "Config",
	Icon:            "config",
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
			WithDescription("search a user or group by name (per the chosen scope); ignored when scope is global").
			WithAutocomplete(map[string]interface{}{
				"function": func(ctx context.Context, input string, values map[string]interface{}) []AutoOption {
					scope, _ := values["scope"].(string)
					if scope == "global" || scope == "" {
						return []AutoOption{}
					}
					db, err := pgdb.GetInstanceCtx(ctx)
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
							Label: name,
						})
					}
					return out
				},
				"view": func(ctx context.Context, id string, values map[string]interface{}) (string, bool) {
					scope, _ := values["scope"].(string)
					idInt, err := strconv.Atoi(id)
					if err != nil {
						return "", false
					}
					db, err := pgdb.GetInstanceCtx(ctx)
					if err != nil {
						return "", false
					}
					var row map[string]interface{}
					switch scope {
					case "user":
						row, err = db.GetOne("SELECT user_name AS name FROM users WHERE id = $1", idInt)
					case "group":
						row, err = db.GetOne("SELECT name FROM user_groups WHERE id = $1", idInt)
					default:
						return "", false
					}
					if err != nil || row == nil {
						return "", false
					}
					name := functions.Coerce[string](row["name"])
					return name, name != ""
				},
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

		// Which named view (ModulePage's layout toggle) is the default for a module's mode, at this row's
		// scope — resolved global < group < user by engine/controllers/config.Resolve, same as theme.
		NewField("module_views", TYPE_TABLE, false).
			WithLabel("Default Views").
			WithDescription("Which view is the default for a module's mode, for anyone at this row's scope").
			WithOption("width", "600px").
			InModes(MODE_VIEW | MODE_CREATE | MODE_EDIT | MODE_SUBMIT).
			TableFieldset([]Field{
				NewField("module", TYPE_STRING, true).
					WithLabel("Module").
					WithOption("widget", "select").
					WithOption("optionsSource", "modules").
					WithOption("width", "220px"),
				NewField("mode", TYPE_SELECT, true).
					WithLabel("Mode").
					WithOptions(modeOptions).
					WithOption("width", "140px"),
				NewField("view", TYPE_STRING, true).
					WithLabel("View name").
					WithDescription(`One of the module's own view names for the chosen mode (e.g. "gallery"); "default" and "system" always work`).
					WithAutocomplete(map[string]interface{}{"function": moduleViewOptions}),
			}).
			TableRowsAddable("module").
			TableData(moduleViewsTableData).
			TableOnSubmit(moduleViewsOnSubmit),
	},
	// Administration module: no access unless explicitly granted (or admin).
	DefaultPermission:    PERMISSION_DENY,
	DefaultPermissionSet: true,
	Rights:               make(map[int]int),
}

func init() { app.RegisterModule(Module, "config") }
