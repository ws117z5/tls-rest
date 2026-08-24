package modulerights

import (
	"tls-rest/go/engine/controllers/auth"
	. "tls-rest/go/engine/controllers/field"
	. "tls-rest/go/engine/controllers/module"
)

// This package registers the two rights-administration modules that back the
// per-mode access model (see auth/resolve.go):
//
//   user_group_rights - which modes a whole group may perform on a module
//   user_rights       - extra modes granted to an individual user
//
// Both store an auth.MODE_* bitmask (list=1 view=2 create=4 edit=8 delete=16)
// in an integer `modes` column. The resolver OR-s a user's group rights and
// their own user rights (plus the module default) to get effective access.

// modeBitOptions describes the individual mode bits so the frontend can render
// the modes bitmask as a set of checkboxes.
func modeBitOptions() []map[string]interface{} {
	// Include both "name" (the shape select-style widgets read) and "label" so
	// whichever key the modes widget consumes shows the mode name.
	return []map[string]interface{}{
		{"name": "List", "label": "List", "value": auth.MODE_LIST},
		{"name": "View", "label": "View", "value": auth.MODE_VIEW},
		{"name": "Create", "label": "Create", "value": auth.MODE_CREATE},
		{"name": "Edit", "label": "Edit", "value": auth.MODE_EDIT},
		{"name": "Delete", "label": "Delete", "value": auth.MODE_DELETE},
	}
}

// modesField builds the shared allowed-modes bitmask field.
func modesField() Field {
	return NewField("modes", TYPE_BITMASK_SELECT, true).
		WithLabel("Allowed Modes").
		WithDescription("Modes this subject may perform on the module").
		WithDefaultValue(0).
		WithValidation("min", 0).
		WithOptions(modeBitOptions)
}

// moduleField builds the shared module-select field. Options are resolved lazily
// from the registered modules at request time (optionsSource:"modules") — reading
// go.config.json here would be empty now that modules are registry-driven, and
// reading the registry eagerly would be empty at package-init (before Register).
func moduleField() Field {
	return NewField("module", TYPE_SELECT, true).
		WithLabel("Module").
		WithDescription("Module these rights apply to").
		WithOption("optionsSource", "modules")
}

// fieldsField builds the shared per-field access control as a TYPE_TABLE.
// Columns are a fieldset: a read-only "field" name plus one checkbox per mode.
// Rows are the fields of the module named by the sibling "module" select. On
// submit, the checkbox rows are folded into the stored JSON
// {"<field>": ["view","edit", ...]} — see auth.ResolveModuleFieldRights.
func fieldsField() Field {
	return NewField("fields", TYPE_TABLE, false).
		WithLabel("Field Access").
		WithDescription("Per-field mode access; leave empty to allow all fields").
		TableFieldset([]Field{
			NewField("field", TYPE_STRING, false).WithLabel("Field").AsReadOnly(),
			NewField("list", TYPE_CHECKBOX, false).WithLabel("List"),
			NewField("view", TYPE_CHECKBOX, false).WithLabel("View"),
			NewField("create", TYPE_CHECKBOX, false).WithLabel("Create"),
			NewField("edit", TYPE_CHECKBOX, false).WithLabel("Edit"),
			NewField("delete", TYPE_CHECKBOX, false).WithLabel("Delete"),
		}).
		// Rows are provided server-side by TableData: the fields of the module
		// chosen in the sibling "module" select (passed as context).
		TableData(func(ctx map[string]interface{}) []map[string]interface{} {
			modID, _ := ctx["module"].(string)
			if modID == "" {
				return nil
			}
			m, ok := RegisteredModules[modID]
			if !ok {
				return nil
			}
			rows := []map[string]interface{}{}
			for _, f := range m.GetFields() {
				if f.Name == "id" {
					continue
				}
				rows = append(rows, map[string]interface{}{"field": f.Name})
			}
			return rows
		}).
		TableOnSubmit(func(rows []map[string]interface{}) interface{} {
			out := map[string][]string{}
			for _, r := range rows {
				name, _ := r["field"].(string)
				if name == "" {
					continue
				}
				modes := []string{}
				for _, m := range []string{"list", "view", "create", "edit", "delete"} {
					if truthy(r[m]) {
						modes = append(modes, m)
					}
				}
				if len(modes) > 0 {
					out[name] = modes
				}
			}
			return out
		})
}

// truthy interprets a checkbox cell value (bool, "true"/"1", 1, etc.).
func truthy(v interface{}) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return t == "true" || t == "1" || t == "on"
	case float64:
		return t != 0
	case int:
		return t != 0
	}
	return false
}

// GroupRightsModule: per-group module rights. Stored group_id as an integer FK
// (INTEGER column) so it joins cleanly to users.user_group; rendered as a select.
var GroupRightsModule = &ModuleAbstract[interface{}]{
	ID:      "user_group_rights",
	Name:    "Group Rights",
	Submenu: "engine",
	Fields: []Field{
		NewField("group_id", TYPE_INT, true).
			WithLabel("User Group").
			WithDescription("Group these rights apply to").
			WithOption("widget", "select").
			WithOption("dataSource", "user_groups").
			WithOption("valueField", "id").
			WithOption("displayField", "name"),
		moduleField(),
		modesField(),
		fieldsField(),
	},
	// Administration module: no access unless explicitly granted (or admin).
	DefaultPermission:    PERMISSION_DENY,
	DefaultPermissionSet: true,
	Rights:               make(map[int]int),
}

// UserRightsModule: extra per-user module rights, additive on top of the user's
// group rights.
var UserRightsModule = &ModuleAbstract[interface{}]{
	ID:      "user_rights",
	Name:    "User Rights",
	Submenu: "engine",
	Fields: []Field{
		NewField("user_id", TYPE_INT, true).
			WithLabel("User").
			WithDescription("User these rights apply to").
			WithOption("widget", "select").
			WithOption("dataSource", "users").
			WithOption("valueField", "id").
			WithOption("displayField", "user_name"),
		moduleField(),
		modesField(),
		fieldsField(),
	},
	// Administration module: no access unless explicitly granted (or admin).
	DefaultPermission:    PERMISSION_DENY,
	DefaultPermissionSet: true,
	Rights:               make(map[int]int),
}

// Register wires this package into the engine (called from go/imports.go).
func Init() {
	GroupRightsModule.Initialize("user_group_rights")
	UserRightsModule.Initialize("user_rights")
}
