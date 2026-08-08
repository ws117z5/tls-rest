package usergroups

import (
	"net/http"
	"time"

	. "tls-rest/go/engine"
)

type UserGroup struct {
	tableName struct{} `pg:"user_groups"`

	ID          int64     `json:"id"`
	UUID        string    `sql:"default:uuid_generate_v4()" json:"uuid"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Active      bool      `json:"active"`
	Created     time.Time `sql:"default:now()" json:"created"`
	Updated     time.Time `sql:"default:now()" json:"updated"`
}

// Initialize the user group module with fieldset configuration
var Module = &ModuleAbstract[interface{}]{
	ID:   "user_groups",
	Name: "User Groups",
	Fields: []Field{
		NewField("name", TYPE_STRING, true).
			WithLabel("Group Name").
			WithDescription("Name of the user group").
			WithValidation("minLength", 2).
			WithValidation("maxLength", 100).
			WithValidation("unique", true),

		NewField("description", TYPE_TEXT, false).
			WithLabel("Description").
			WithDescription("Description of the group's purpose").
			WithValidation("maxLength", 500).
			NonSortable(),

		NewField("active", TYPE_CHECKBOX, false).
			WithLabel("Active").
			WithDescription("Whether the group is active").
			WithDefaultValue(true),

		NewField("members", TYPE_JSON, false).
			WithLabel("Members").
			WithDescription("List of group members").
			WithSQL(`
				SELECT u.user_name, ugm.role, ugm.created as joined_at
				FROM user_group_members ugm
				JOIN users u ON u.id = ugm.user_id
				WHERE ugm.group_id = ${id}
			`).
			AsReadOnly().
			NonSortable().
			NonSearchable(),

		NewField("permissions", TYPE_TABLE, false).
			WithLabel("Group Permissions").
			WithDescription("Permissions assigned to this group").
			WithTableQuery(`
				SELECT m.name as module_name, ugr.rights, ugr.created_at as granted_at
				FROM user_group_rights ugr
				JOIN modules m ON m.id = ugr.module_id
				WHERE ugr.group_id = ?
			`).
			WithTableColumns([]string{"module_name", "rights", "granted_at"}).
			WithTableEditable(true).
			WithTableSubmitFunction("updateGroupPermissions").
			WithTableRowActions([]map[string]interface{}{
				{"label": "Edit Rights", "action": "editRights", "icon": "key"},
				{"label": "Remove", "action": "removePermission", "icon": "trash"},
			}),

		NewField("member_count", TYPE_TABLE, false).
			WithLabel("Member Statistics").
			WithDescription("Statistics about group membership").
			WithTableQuery(`
				SELECT 
					ugm.role,
					COUNT(*) as count,
					MAX(ugm.created) as last_joined
				FROM user_group_members ugm
				WHERE ugm.group_id = ?
				GROUP BY ugm.role
			`).
			WithTableColumns([]string{"role", "count", "last_joined"}),

		NewField("available_permissions", TYPE_TABLE, false).
			WithLabel("Available Permissions").
			WithDescription("All available modules and their permissions").
			WithTableData([]map[string]interface{}{
				{"module": "users", "description": "User management", "available_rights": "view, create, edit, delete"},
				{"module": "user_groups", "description": "Group management", "available_rights": "view, create, edit, delete"},
				{"module": "posts", "description": "Post management", "available_rights": "view, create, edit, delete"},
				{"module": "system", "description": "System administration", "available_rights": "view, config, backup, restore"},
			}).
			WithTableColumns([]string{"module", "description", "available_rights"}),

		NewField("created", TYPE_DATE_TIME, false).
			WithLabel("Created").
			WithSQL("now()").
			AsReadOnly().
			NonFilterable().
			WithMode(MODE_LIST | MODE_VIEW),

		NewField("updated", TYPE_DATE_TIME, false).
			WithLabel("Last Updated").
			WithSQL("now()").
			AsReadOnly().
			NonFilterable().
			WithMode(MODE_LIST | MODE_VIEW),
	},
	Rights: make(map[int]int),
}

func init() {
	// Initialize the module - this creates the controller and registers with fieldset handler
	Module.Initialize("user_groups")
}

// CRUD operations are now handled automatically by the module system
// Routes are automatically registered via RegisterModuleRoutes() function

func AddMember(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement add member to group functionality
}

func RemoveMember(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement remove member from group functionality
}

func UpdateMemberRole(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement update member role functionality
}

func AssignPermission(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement assign permission to group functionality
}

func RevokePermission(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement revoke permission from group functionality
}
