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

		// Admin groups bypass all mode and record-access checks. The group's id
		// is the access level (0/none = unauthorized, higher = more privileged),
		// so no separate level column is needed.
		NewField("is_admin", TYPE_CHECKBOX, false).
			WithLabel("Administrator").
			WithDescription("Members of this group have unrestricted access").
			WithDefaultValue(false),
	},
	// Administration module: no access unless explicitly granted (or admin).
	DefaultPermission:    PERMISSION_DENY,
	DefaultPermissionSet: true,
	Rights:               make(map[int]int),
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
