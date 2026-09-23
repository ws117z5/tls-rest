// Package externalconfig is the admin module mapping an OAuth provider to
// the user group and module rights new sign-ins from it get — read by
// auth/oauth.go on new-user creation.
package externalconfig

import (
	"context"
	"net/http"

	"tls-rest/go/app"
	"tls-rest/go/engine/controllers/auth"
	"tls-rest/go/engine/controllers/db/pgdb"
	. "tls-rest/go/engine/controllers/field"
	"tls-rest/go/engine/controllers/functions"
	. "tls-rest/go/engine/controllers/module"
)

// grantedModes: everything but delete — ordinary app-user accounts, not admins.
const grantedModes = auth.MODE_LIST | auth.MODE_VIEW | auth.MODE_CREATE | auth.MODE_EDIT

func providerOptions() []map[string]interface{} {
	return []map[string]interface{}{
		{"value": "google", "name": "Google"},
		{"value": "github", "name": "GitHub"},
		{"value": "facebook", "name": "Facebook"},
		{"value": "vk", "name": "VK"},
		{"value": "x", "name": "X (Twitter)"},
	}
}

// provisionGroupRights OR-s grantedModes into group's existing rights on
// module, never revoking anything already granted.
func provisionGroupRights(ctx context.Context, groupID int, module string) error {
	if groupID <= 0 || module == "" {
		return nil
	}
	db, err := pgdb.GetInstanceCtx(ctx)
	if err != nil {
		return err
	}
	_, err = db.Exec(`
		INSERT INTO user_group_rights (group_id, module, modes) VALUES ($1, $2, $3)
		ON CONFLICT (group_id, module) DO UPDATE SET modes = user_group_rights.modes | EXCLUDED.modes
	`, groupID, module, grantedModes)
	return err
}

// afterSave provisions group rights for app_module and every related_modules
// entry, then bumps the rights epoch so any already-active session in that
// group picks the new rights up on its next request.
func afterSave(r *http.Request, data map[string]interface{}) (map[string]interface{}, error) {
	groupID := functions.Int(data["group_id"])
	if appModule, _ := data["app_module"].(string); appModule != "" {
		if err := provisionGroupRights(r.Context(), groupID, appModule); err != nil {
			return data, err
		}
	}
	if related, ok := data["related_modules"].([]interface{}); ok {
		for _, m := range related {
			if name, _ := m.(string); name != "" {
				if err := provisionGroupRights(r.Context(), groupID, name); err != nil {
					return data, err
				}
			}
		}
	}
	if OnRightsChange != nil {
		OnRightsChange()
	}
	return data, nil
}

var Module = &ModuleAbstract[interface{}]{
	ID:      "external_config",
	Name:    "External Apps",
	Icon:    "external",
	Submenu: "External",
	Fields: []Field{
		NewField("app_module", TYPE_SELECT, true).
			WithLabel("App Module").
			WithDescription("The external app's primary module, e.g. words").
			WithOption("optionsSource", "modules").
			WithValueWidth(300),

		NewField("provider", TYPE_SELECT, true).
			WithLabel("OAuth Provider").
			WithDescription("Identifies this app's sign-ins (matches the OAuth callback provider)").
			WithOptions(providerOptions).
			WithValueWidth(300),

		NewField("group_id", TYPE_INT, true).
			WithLabel("User Group").
			WithDescription("New users signing in via this provider join this group").
			WithOption("widget", "select").
			WithOption("dataSource", "user_groups").
			WithOption("valueField", "id").
			WithOption("displayField", "name").
			WithValueWidth(300),

		NewField("related_modules", TYPE_TABLE, false).
			WithLabel("Related Modules").
			WithDescription("Other modules this app's group needs rights to, besides App Module").
			WithOption("width", "400px").
			InModes(MODE_VIEW | MODE_CREATE | MODE_EDIT | MODE_SUBMIT).
			TableFieldset([]Field{
				NewField("module", TYPE_STRING, true).
					WithLabel("Module").
					WithOption("widget", "select").
					WithOption("optionsSource", "modules").
					WithOption("width", "300px"),
			}).
			TableRowsAddable("module").
			TableData(func(ctx context.Context, data map[string]interface{}) []map[string]interface{} {
				id := functions.Int(data["id"])
				if id <= 0 {
					return nil
				}
				db, err := pgdb.GetInstanceCtx(ctx)
				if err != nil {
					return nil
				}
				rows, err := db.GetAll(
					`SELECT jsonb_array_elements_text(related_modules) AS "module" FROM external_config WHERE id = $1`,
					id)
				if err != nil {
					return nil
				}
				return rows
			}).
			TableOnSubmit(func(rows []map[string]interface{}) interface{} {
				names := []interface{}{}
				for _, row := range rows {
					if name, _ := row["module"].(string); name != "" {
						names = append(names, name)
					}
				}
				return names
			}),
	},
	AfterFieldset: afterSave,
	// Administration module: no access unless explicitly granted (or admin).
	DefaultPermission:    PERMISSION_DENY,
	DefaultPermissionSet: true,
	Rights:               make(map[int]int),
}

func init() {
	app.RegisterModule(Module, "external_config")
}
