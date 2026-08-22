package modulerights

import (
	config "tls-rest/go/constants"
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

// moduleSelectOptions lists the declared modules (from go.config.json) as
// select options {value:name, label:description||name} for the "module" field.
func moduleSelectOptions() []map[string]interface{} {
	opts := make([]map[string]interface{}, 0, len(config.Config.Modules))
	for _, m := range config.Config.Modules {
		label := m.Description
		if label == "" {
			label = m.Name
		}
		// The frontend Select renders option.name for the display text, so the
		// display goes under "name" (not "label").
		opts = append(opts, map[string]interface{}{"value": m.Name, "name": label})
	}
	return opts
}

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
	return NewField("modes", TYPE_INT, true).
		WithLabel("Allowed Modes").
		WithDescription("Modes this subject may perform on the module").
		WithDefaultValue(0).
		WithValidation("min", 0).
		WithOption("widget", "bitmask").
		WithOption("bits", modeBitOptions())
}

// moduleField builds the shared module-select field.
func moduleField() Field {
	return NewField("module", TYPE_STRING, true).
		WithLabel("Module").
		WithDescription("Module these rights apply to").
		WithOption("widget", "select").
		WithOption("options", moduleSelectOptions())
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
	},
	// Administration module: no access unless explicitly granted (or admin).
	DefaultPermission:    PERMISSION_DENY,
	DefaultPermissionSet: true,
	Rights:               make(map[int]int),
}

func Init() {
	GroupRightsModule.Initialize("user_group_rights")
	UserRightsModule.Initialize("user_rights")
}
