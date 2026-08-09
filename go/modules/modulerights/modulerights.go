package modulerights

import (
	"time"

	config "tls-rest/go/constants"
	. "tls-rest/go/engine"
	"tls-rest/go/lib/auth"
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

// UserGroupRight is one group grant.
type UserGroupRight struct {
	tableName struct{} `pg:"user_group_rights"`

	ID      int64     `json:"id"`
	GroupID int64     `json:"group_id"`
	Module  string    `json:"module"`
	Modes   int       `json:"modes"`
	Created time.Time `sql:"default:now()" json:"created"`
	Updated time.Time `sql:"default:now()" json:"updated"`
}

// UserRight is one per-user grant.
type UserRight struct {
	tableName struct{} `pg:"user_rights"`

	ID      int64     `json:"id"`
	UserID  int64     `json:"user_id"`
	Module  string    `json:"module"`
	Modes   int       `json:"modes"`
	Created time.Time `sql:"default:now()" json:"created"`
	Updated time.Time `sql:"default:now()" json:"updated"`
}

// moduleSelectOptions lists the declared modules (from go.config.json) as
// select options {value:name, label:description||name} for the "module" field.
func moduleSelectOptions() []map[string]interface{} {
	opts := make([]map[string]interface{}, 0, len(config.Config.Modules))
	for _, m := range config.Config.Modules {
		label := m.Description
		if label == "" {
			label = m.Name
		}
		opts = append(opts, map[string]interface{}{"value": m.Name, "label": label})
	}
	return opts
}

// modeBitOptions describes the individual mode bits so the frontend can render
// the modes bitmask as a set of checkboxes.
func modeBitOptions() []map[string]interface{} {
	return []map[string]interface{}{
		{"label": "List", "value": auth.MODE_LIST},
		{"label": "View", "value": auth.MODE_VIEW},
		{"label": "Create", "value": auth.MODE_CREATE},
		{"label": "Edit", "value": auth.MODE_EDIT},
		{"label": "Delete", "value": auth.MODE_DELETE},
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
	ID:   "user_group_rights",
	Name: "Group Rights",
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
	ID:   "user_rights",
	Name: "User Rights",
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

func init() {
	GroupRightsModule.Initialize("user_group_rights")
	UserRightsModule.Initialize("user_rights")
}
