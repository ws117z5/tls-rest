package modulerights

import (
	"context"
	"encoding/json"

	"tls-rest/go/app"
	"tls-rest/go/engine/controllers/auth"
	"tls-rest/go/engine/controllers/db/pgdb"
	. "tls-rest/go/engine/controllers/field"
	"tls-rest/go/engine/controllers/functions"
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
		{"name": "Filters", "label": "Filters", "value": auth.MODE_FILTERS},
	}
}

// modesField builds the shared allowed-modes bitmask field.
func modesField() Field {
	return NewField("modes", TYPE_BITMASK_SELECT, true).
		WithLabel("Allowed Modes").
		WithDescription("Modes this subject may perform on the module").
		WithDefault(0).
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

// parseFieldGrants unmarshals a stored `fields` grant value (a JSON string or
// []byte holding {"<field>": ["view","edit", ...]}) into a map. Anything it
// can't parse yields an empty map, meaning "no per-field grants recorded".
func parseFieldGrants(v interface{}) map[string][]string {
	out := map[string][]string{}
	var raw []byte
	switch t := v.(type) {
	case nil:
		return out
	case string:
		raw = []byte(t)
	case []byte:
		raw = t
	default:
		return out
	}
	if len(raw) == 0 {
		return out
	}
	_ = json.Unmarshal(raw, &out)
	return out
}

// parseSpecialRights unmarshals a stored `special_rights` value (a JSON
// string/[]byte array of right ids) into a slice; unparseable yields none.
func parseSpecialRights(v interface{}) []string {
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

// specialRightsField shows one checkbox per right the sibling "module"
// select's module declares, pre-filled from the record's stored grants.
func specialRightsField(rightsTable string) Field {
	return NewField("special_rights", TYPE_TABLE, false).
		WithLabel("Special Rights").
		WithDescription("Custom permissions the module declares (e.g. a gated button or endpoint)").
		TableFieldset([]Field{
			NewField("id", TYPE_STRING, false).WithLabel("Key").AsReadOnly(),
			NewField("name", TYPE_STRING, false).WithLabel("Right").AsReadOnly(),
			NewField("granted", TYPE_CHECKBOX, false).WithLabel("Granted"),
		}).
		TableData(func(ctx context.Context, data map[string]interface{}) []map[string]interface{} {
			modID, _ := data["module"].(string)
			if modID == "" {
				return nil
			}
			m, ok := RegisteredModules[modID]
			if !ok {
				return nil
			}
			rights := m.GetSpecialRights()
			if len(rights) == 0 {
				return nil
			}

			granted := map[string]bool{}
			if recID := functions.Int(data["id"]); recID > 0 {
				if db, err := pgdb.GetInstanceCtx(ctx); err == nil {
					if row, e := db.GetOne(
						"SELECT special_rights FROM "+rightsTable+" WHERE id = $1", recID,
					); e == nil && row != nil {
						for _, id := range parseSpecialRights(row["special_rights"]) {
							granted[id] = true
						}
					}
				}
			}

			rows := []map[string]interface{}{}
			for _, sr := range rights {
				rows = append(rows, map[string]interface{}{
					"id":      sr.ID,
					"name":    sr.Name,
					"granted": granted[sr.ID],
				})
			}
			return rows
		}).
		TableOnSubmit(func(rows []map[string]interface{}) interface{} {
			out := []interface{}{}
			for _, r := range rows {
				id, _ := r["id"].(string)
				if id != "" && functions.Truthy(r["granted"]) {
					out = append(out, id)
				}
			}
			return out
		})
}

// fieldsField builds the shared per-field access control as a TYPE_TABLE.
// Columns are a fieldset: a read-only "field" name plus one checkbox per mode.
// Rows are the fields of the module named by the sibling "module" select, with
// each mode checkbox pre-filled from the record's stored grants (loaded from
// rightsTable by row id). On submit, the checkbox rows are folded back into the
// stored JSON {"<field>": ["view","edit", ...]} — see auth.ResolveModuleFieldRights.
func fieldsField(rightsTable string) Field {
	return NewField("fields", TYPE_TABLE, false).
		WithLabel("Field Access").
		WithDescription("Per-field mode access; leave empty to allow all fields").
		// Checking a mode in the sibling "modes" (Allowed Modes) bitmask checks
		// that column for every field row here; unchecking clears them.
		WithOption("syncColumnsFromBitmask", "modes").
		TableFieldset([]Field{
			NewField("field", TYPE_STRING, false).WithLabel("Field").AsReadOnly(),
			NewField("list", TYPE_CHECKBOX, false).WithLabel("List"),
			NewField("view", TYPE_CHECKBOX, false).WithLabel("View"),
			NewField("create", TYPE_CHECKBOX, false).WithLabel("Create"),
			NewField("edit", TYPE_CHECKBOX, false).WithLabel("Edit"),
			NewField("delete", TYPE_CHECKBOX, false).WithLabel("Delete"),
		}).
		// Rows are provided server-side by TableData: the fields of the module
		// chosen in the sibling "module" select (passed as context), each with
		// its mode checkboxes filled from this record's stored grants.
		TableData(func(ctx context.Context, data map[string]interface{}) []map[string]interface{} {
			modID, _ := data["module"].(string)
			if modID == "" {
				return nil
			}
			m, ok := RegisteredModules[modID]
			if !ok {
				return nil
			}

			// Load the record's stored per-field grants, if it's an existing row.
			stored := map[string][]string{}
			if recID := functions.Int(data["id"]); recID > 0 {
				if db, err := pgdb.GetInstanceCtx(ctx); err == nil {
					if row, e := db.GetOne(
						"SELECT fields FROM "+rightsTable+" WHERE id = $1", recID,
					); e == nil && row != nil {
						stored = parseFieldGrants(row["fields"])
					}
				}
			}
			has := func(field, mode string) bool {
				for _, g := range stored[field] {
					if g == mode {
						return true
					}
				}
				return false
			}

			rows := []map[string]interface{}{}
			for _, f := range m.GetFields() {
				if f.Name == "id" {
					continue
				}
				rows = append(rows, map[string]interface{}{
					"field":  f.Name,
					"list":   has(f.Name, "list"),
					"view":   has(f.Name, "view"),
					"create": has(f.Name, "create"),
					"edit":   has(f.Name, "edit"),
					"delete": has(f.Name, "delete"),
				})
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
					if functions.Truthy(r[m]) {
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

// parseFilterFieldGrants unmarshals a stored `filter_fields` value (a JSON
// array of allowed filter names) into a set. Anything unparseable yields an
// empty set, meaning "no per-filter grants recorded".
func parseFilterFieldGrants(v interface{}) map[string]bool {
	out := map[string]bool{}
	var raw []byte
	switch t := v.(type) {
	case string:
		raw = []byte(t)
	case []byte:
		raw = t
	default:
		return out
	}
	var names []string
	if json.Unmarshal(raw, &names) == nil {
		for _, n := range names {
			out[n] = true
		}
	}
	return out
}

// filterFieldsField is fieldsField's counterpart for filters.go's own
// fieldset (a module's declared list filters, separate from its data fields).
func filterFieldsField(rightsTable string) Field {
	return NewField("filter_fields", TYPE_TABLE, false).
		WithLabel("Filter Access").
		WithDescription("Which declared filters may be used; leave empty to allow all").
		WithOption("syncColumnsFromBitmask", "modes").
		TableFieldset([]Field{
			NewField("filter", TYPE_STRING, false).WithLabel("Filter").AsReadOnly(),
			NewField("filters", TYPE_CHECKBOX, false).WithLabel("Filters"),
		}).
		TableData(func(ctx context.Context, data map[string]interface{}) []map[string]interface{} {
			modID, _ := data["module"].(string)
			if modID == "" {
				return nil
			}
			m, ok := RegisteredModules[modID]
			if !ok || m.GetFilters() == nil {
				return nil
			}

			granted := map[string]bool{}
			if recID := functions.Int(data["id"]); recID > 0 {
				if db, err := pgdb.GetInstanceCtx(ctx); err == nil {
					if row, e := db.GetOne(
						"SELECT filter_fields FROM "+rightsTable+" WHERE id = $1", recID,
					); e == nil && row != nil {
						granted = parseFilterFieldGrants(row["filter_fields"])
					}
				}
			}

			rows := []map[string]interface{}{}
			for _, f := range m.GetFilters().Fields {
				rows = append(rows, map[string]interface{}{
					"filter":  f.Name,
					"filters": granted[f.Name],
				})
			}
			return rows
		}).
		TableOnSubmit(func(rows []map[string]interface{}) interface{} {
			out := []string{}
			for _, r := range rows {
				name, _ := r["filter"].(string)
				if name != "" && functions.Truthy(r["filters"]) {
					out = append(out, name)
				}
			}
			return out
		})
}

// groupRightsFilters declares GET /user_group_rights's list filters: group
// (by name, via subquery) and module (direct substring match).
func groupRightsFilters() *ListFilters {
	return NewFieldset(
		NewFilter("group", TYPE_STRING).
			WithLabel("Group").
			Contains().
			WithSQLWhere("group_id IN (SELECT id FROM user_groups WHERE name ILIKE %s)"),
		NewFilter("module", TYPE_STRING).
			WithLabel("Module").
			Contains(),
	)
}

// GroupRightsModule holds per-group module rights.
var GroupRightsModule = &ModuleAbstract[interface{}]{
	ID:              "user_group_rights",
	RightsAffecting: true,
	Name:            "Group Rights",
	Icon:            "group-rights",
	Submenu:         "engine",
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
		fieldsField("user_group_rights"),
		filterFieldsField("user_group_rights"),
		specialRightsField("user_group_rights"),
	},
	Filters: groupRightsFilters(),
	// Administration module: no access unless explicitly granted (or admin).
	DefaultPermission:    PERMISSION_DENY,
	DefaultPermissionSet: true,
	Rights:               make(map[int]int),
}

// UserRightsModule holds extra per-user module rights, additive on top of group rights.
var UserRightsModule = &ModuleAbstract[interface{}]{
	ID:              "user_rights",
	RightsAffecting: true,
	Name:            "User Rights",
	Icon:            "user-rights",
	Submenu:         "engine",
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
		fieldsField("user_rights"),
		filterFieldsField("user_rights"),
		specialRightsField("user_rights"),
	},
	// Administration module: no access unless explicitly granted (or admin).
	DefaultPermission:    PERMISSION_DENY,
	DefaultPermissionSet: true,
	Rights:               make(map[int]int),
}

func init() {
	app.RegisterModule(GroupRightsModule, "user_group_rights")
	app.RegisterModule(UserRightsModule, "user_rights")
}
