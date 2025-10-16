package modulerights

import (
	"net/http"
	"time"

	. "github.com/ws117z5/tls-rest/go/lib/module"
)

// ModuleRight represents a permission assignment
type ModuleRight struct {
	tableName struct{} `pg:"module_rights"`

	ID          int64     `json:"id"`
	ModuleID    string    `json:"module_id"`
	SubjectID   string    `json:"subject_id"`   // User ID or Group ID
	SubjectType string    `json:"subject_type"` // "user" or "group"
	Rights      int       `json:"rights"`       // Bitfield for permissions
	GrantedBy   string    `json:"granted_by"`   // Who granted these rights
	Created     time.Time `sql:"default:now()" json:"created"`
	Updated     time.Time `sql:"default:now()" json:"updated"`
}

// Rights constants are imported from the module package

// Initialize the module rights module with fieldset configuration
var Module = &ModuleAbstract[interface{}]{
	ID:   "module_rights",
	Name: "Module Rights Management",
	Fields: []Field{
		NewField("module_id", TYPE_SELECT, true).
			WithLabel("Module").
			WithDescription("Target module for permissions").
			WithOption("dataSource", "modules").
			WithOption("valueField", "id").
			WithOption("displayField", "name"),

		NewField("subject_id", TYPE_STRING, true).
			WithLabel("Subject ID").
			WithDescription("User ID or Group ID").
			WithValidation("required", true),

		NewField("subject_type", TYPE_SELECT, true).
			WithLabel("Subject Type").
			WithDescription("Whether this applies to a user or group").
			WithOption("user", "User").
			WithOption("group", "Group"),

		NewField("subject_info", TYPE_TABLE, false).
			WithLabel("Subject Details").
			WithDescription("Information about the user or group").
			WithTableQuery(`
				CASE 
					WHEN $2 = 'user' THEN (
						SELECT json_build_object(
							'type', 'user',
							'name', first_name || ' ' || last_name,
							'email', email,
							'active', 'Yes'
						)::text
						FROM users WHERE id::text = $1
					)
					WHEN $2 = 'group' THEN (
						SELECT json_build_object(
							'type', 'group',
							'name', name,
							'description', description,
							'active', CASE WHEN active THEN 'Yes' ELSE 'No' END
						)::text
						FROM user_groups WHERE id::text = $1
					)
				END as info
			`).
			WithTableColumns([]string{"info"}),

		NewField("rights", TYPE_INT, true).
			WithLabel("Rights Value").
			WithDescription("Bitfield value for permissions").
			WithValidation("min", 0).
			WithValidation("max", RIGHT_ALL),

		NewField("rights_breakdown", TYPE_TABLE, false).
			WithLabel("Permissions Breakdown").
			WithDescription("Human-readable breakdown of permissions").
			WithTableData([]map[string]interface{}{
				{"permission": "View", "bit_value": RIGHT_VIEW, "description": "Can view records"},
				{"permission": "Create", "bit_value": RIGHT_CREATE, "description": "Can create new records"},
				{"permission": "Edit", "bit_value": RIGHT_EDIT, "description": "Can modify existing records"},
				{"permission": "Delete", "bit_value": RIGHT_DELETE, "description": "Can delete records"},
				{"permission": "Admin", "bit_value": RIGHT_ADMIN, "description": "Full administrative access"},
			}).
			WithTableColumns([]string{"permission", "bit_value", "description"}),

		NewField("granted_by", TYPE_STRING, false).
			WithLabel("Granted By").
			WithDescription("User who granted these permissions").
			AsReadOnly().
			WithMode(MODE_VIEW | MODE_LIST),

		NewField("effective_permissions", TYPE_TABLE, false).
			WithLabel("Effective Permissions").
			WithDescription("Combined permissions from direct assignment and groups").
			WithTableQuery(`
				WITH user_direct_rights AS (
					SELECT mr.module_id, mr.rights as direct_rights
					FROM module_rights mr
					WHERE mr.subject_id = $1 AND mr.subject_type = 'user'
				),
				user_group_rights AS (
					SELECT mr.module_id, BIT_OR(mr.rights) as group_rights
					FROM module_rights mr
					JOIN user_group_members ugm ON ugm.group_id::text = mr.subject_id
					WHERE ugm.user_id::text = $1 AND mr.subject_type = 'group'
					GROUP BY mr.module_id
				)
				SELECT 
					COALESCE(udr.module_id, ugr.module_id) as module_name,
					COALESCE(udr.direct_rights, 0) as direct_rights,
					COALESCE(ugr.group_rights, 0) as inherited_rights,
					(COALESCE(udr.direct_rights, 0) | COALESCE(ugr.group_rights, 0)) as total_rights
				FROM user_direct_rights udr
				FULL OUTER JOIN user_group_rights ugr ON udr.module_id = ugr.module_id
			`).
			WithTableColumns([]string{"module_name", "direct_rights", "inherited_rights", "total_rights"}),

		NewField("audit_log", TYPE_TABLE, false).
			WithLabel("Rights Changes").
			WithDescription("History of permission changes").
			WithTableQuery(`
				SELECT 
					'Rights Modified' as action,
					'Rights changed from ' || old_rights || ' to ' || new_rights as details,
					modified_by as changed_by,
					created_at as timestamp
				FROM module_rights_audit
				WHERE subject_id = $1 AND subject_type = $2
				ORDER BY created_at DESC
				LIMIT 20
			`).
			WithTableColumns([]string{"action", "details", "changed_by", "timestamp"}),
	},
	Rights: make(map[int]int),
}

func init() {
	// Initialize the module - this creates the controller and registers with fieldset handler
	Module.Initialize("module_rights")
}

// CRUD operations are now handled automatically by the module system
// Routes are automatically registered via RegisterModuleRoutes() function

// Helper functions are available from the module package via dot-import

// Custom API handlers for rights management
func GrantRights(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement grant rights functionality
	// This would handle granting specific rights to a user/group
}

func RevokeRights(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement revoke rights functionality
	// This would handle removing specific rights from a user/group
}

func GetUserEffectiveRights(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement get effective rights for a user
	// This would calculate combined rights from direct assignment and groups
}

func GetGroupRights(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement get all rights for a group
	// This would return all rights assigned to a specific group
}

func BulkUpdateRights(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement bulk update rights functionality
	// This would handle updating multiple rights assignments at once
}
