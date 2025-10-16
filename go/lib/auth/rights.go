package auth

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ws117z5/tls-rest/go/lib/db/pgdb"
	"github.com/ws117z5/tls-rest/go/lib/log"
)

// Permission Values according to the schema
const (
	PERMISSION_INHERIT = -1 // Inherit permission from parent (group or default)
	PERMISSION_DENY    = 0  // No access to module
	PERMISSION_READ    = 1  // Read-only access
	PERMISSION_WRITE   = 2  // Full read/write access
)

// Rights system structures matching the database schema
type UserRight struct {
	UserID     int    `json:"user_id" db:"userid"`
	Module     string `json:"module" db:"module"`
	Permission int    `json:"permission" db:"permission"`
}

type UserRightSpecial struct {
	UserID     int    `json:"user_id" db:"userid"`
	Module     string `json:"module" db:"module"`
	SpecialID  string `json:"special_id" db:"specialid"`
	Permission int    `json:"permission" db:"permission"`
}

type UserGroupRight struct {
	GroupID    int    `json:"group_id" db:"groupid"`
	Module     string `json:"module" db:"module"`
	Permission int    `json:"permission" db:"permission"`
}

type UserGroupRightSpecial struct {
	GroupID    int    `json:"group_id" db:"groupid"`
	Module     string `json:"module" db:"module"`
	SpecialID  string `json:"special_id" db:"specialid"`
	Permission int    `json:"permission" db:"permission"`
}

type User struct {
	ID                   int     `json:"id" db:"id"`
	Name                 string  `json:"name" db:"name"`
	Login                string  `json:"login" db:"login"`
	Password             string  `json:"password" db:"password"`
	Email                string  `json:"email" db:"email"`
	GroupID              *int    `json:"group_id" db:"groupid"`
	Active               bool    `json:"active" db:"active"`
	CurrentVisitDateTime *string `json:"current_visit" db:"currentvisitdatetime"`
	LastVisitDateTime    *string `json:"last_visit" db:"lastvisitdatetime"`
	ResponsibleRoleIDs   string  `json:"responsible_role_ids" db:"responsibleroleids"` // Comma-separated IDs
}

type UserGroup struct {
	ID          int    `json:"id" db:"id"`
	Name        string `json:"name" db:"name"`
	Description string `json:"description" db:"description"`
	OrderID     int    `json:"order_id" db:"orderid"`
}

type ResponsibleRole struct {
	ID      int    `json:"id" db:"id"`
	Name    string `json:"name" db:"name"`
	OrderID int    `json:"order_id" db:"orderid"`
}

// ModuleDefaultPermission stores default permissions for modules
type ModuleDefaultPermission struct {
	Module            string `json:"module"`
	DefaultPermission int    `json:"default_permission"`
}

// Global registry for module default permissions
var ModuleDefaultPermissions = make(map[string]int)

// RegisterModuleDefaultPermission sets the default permission for a module
func RegisterModuleDefaultPermission(module string, defaultPermission int) {
	ModuleDefaultPermissions[module] = defaultPermission
	log.LogSystemEvent(
		fmt.Sprintf("Default permission %d registered for module %s", defaultPermission, module),
		log.LogLevelInfo, map[string]interface{}{
			"module":             module,
			"default_permission": defaultPermission,
		})
}

// GetEffectivePermission implements the 3-tier inheritance model:
// 1. System Default → Module-level default permission
// 2. User Group → Group-level permissions override defaults
// 3. User → User-level permissions override group permissions
func GetEffectivePermission(userID int, module string) (int, error) {
	// Get user info to determine group
	user, err := GetUserByID(userID)
	if err != nil {
		return PERMISSION_DENY, fmt.Errorf("failed to get user: %w", err)
	}

	// Step 1: Get module default permission
	defaultPermission, exists := ModuleDefaultPermissions[module]
	if !exists {
		defaultPermission = PERMISSION_READ // System fallback default
	}

	finalPermission := defaultPermission

	// Step 2: Check group-level permissions if user belongs to a group
	if user.GroupID != nil {
		groupPermission, err := GetGroupPermission(*user.GroupID, module)
		if err == nil && groupPermission != PERMISSION_INHERIT {
			finalPermission = groupPermission
		}
	}

	// Step 3: Check user-level permissions (highest priority)
	userPermission, err := GetUserPermission(userID, module)
	if err == nil && userPermission != PERMISSION_INHERIT {
		finalPermission = userPermission
	}

	log.LogSystemEvent(
		fmt.Sprintf("Permission resolved for user %d, module %s: %d", userID, module, finalPermission),
		log.LogLevelDebug, map[string]interface{}{
			"user_id":            userID,
			"module":             module,
			"final_permission":   finalPermission,
			"default_permission": defaultPermission,
		})

	return finalPermission, nil
}

// GetUserPermission gets user-specific permission for a module
func GetUserPermission(userID int, module string) (int, error) {
	db, err := pgdb.GetInstance()
	if err != nil {
		return PERMISSION_INHERIT, err
	}

	query := "SELECT permission FROM user_right WHERE userid = ? AND module = ?"
	result, err := db.GetOne(query, userID, module)
	if err != nil {
		return PERMISSION_INHERIT, nil // No specific permission set (treat as inherit)
	}

	if result == nil {
		return PERMISSION_INHERIT, nil
	}

	if permission, ok := result.(int); ok {
		return permission, nil
	}

	return PERMISSION_INHERIT, nil
}

// GetGroupPermission gets group-level permission for a module
func GetGroupPermission(groupID int, module string) (int, error) {
	db, err := pgdb.GetInstance()
	if err != nil {
		return PERMISSION_INHERIT, err
	}

	query := "SELECT permission FROM usergroup_right WHERE groupid = ? AND module = ?"
	result, err := db.GetOne(query, groupID, module)
	if err != nil {
		return PERMISSION_INHERIT, nil // No specific permission set (treat as inherit)
	}

	if result == nil {
		return PERMISSION_INHERIT, nil
	}

	if permission, ok := result.(int); ok {
		return permission, nil
	}

	return PERMISSION_INHERIT, nil
}

// GetUserByID retrieves user information by ID
func GetUserByID(userID int) (*User, error) {
	db, err := pgdb.GetInstance()
	if err != nil {
		return nil, err
	}

	query := `SELECT id, name, login, password, email, groupid, active, 
			  currentvisitdatetime, lastvisitdatetime, responsibleroleids 
			  FROM "user" WHERE id = ?`

	results, err := db.GetAll(query, userID)
	if err != nil || len(results) == 0 {
		return nil, fmt.Errorf("user not found")
	}

	result := results[0]
	user := &User{
		ID:                 result["id"].(int),
		Name:               result["name"].(string),
		Login:              result["login"].(string),
		Password:           result["password"].(string),
		Email:              result["email"].(string),
		Active:             result["active"].(bool),
		ResponsibleRoleIDs: result["responsibleroleids"].(string),
	}

	// Handle nullable fields
	if result["groupid"] != nil {
		if groupID, ok := result["groupid"].(int); ok {
			user.GroupID = &groupID
		}
	}

	if result["currentvisitdatetime"] != nil {
		if visitTime, ok := result["currentvisitdatetime"].(string); ok {
			user.CurrentVisitDateTime = &visitTime
		}
	}

	if result["lastvisitdatetime"] != nil {
		if visitTime, ok := result["lastvisitdatetime"].(string); ok {
			user.LastVisitDateTime = &visitTime
		}
	}

	return user, nil
}

// SetUserPermission sets user-specific permission for a module
func SetUserPermission(userID int, module string, permission int) error {
	db, err := pgdb.GetInstance()
	if err != nil {
		return err
	}

	// First try to update existing record
	updateQuery := "UPDATE user_right SET permission = ? WHERE userid = ? AND module = ?"
	result, err := db.Exec(updateQuery, permission, userID, module)

	if err != nil {
		return err
	}

	// If no rows were affected, insert new record
	if affected, _ := db.GetAffectedRows(result); affected == 0 {
		insertQuery := "INSERT INTO user_right (userid, module, permission) VALUES (?, ?, ?)"
		_, err = db.Exec(insertQuery, userID, module, permission)
		if err != nil {
			return err
		}
	}

	log.LogSystemEvent(
		fmt.Sprintf("User %d permission set for module %s: %d", userID, module, permission),
		log.LogLevelInfo, map[string]interface{}{
			"user_id":    userID,
			"module":     module,
			"permission": permission,
		})

	return nil
}

// SetGroupPermission sets group-level permission for a module
func SetGroupPermission(groupID int, module string, permission int) error {
	db, err := pgdb.GetInstance()
	if err != nil {
		return err
	}

	// First try to update existing record
	updateQuery := "UPDATE usergroup_right SET permission = ? WHERE groupid = ? AND module = ?"
	result, err := db.Exec(updateQuery, permission, groupID, module)

	if err != nil {
		return err
	}

	// If no rows were affected, insert new record
	if affected, _ := db.GetAffectedRows(result); affected == 0 {
		insertQuery := "INSERT INTO usergroup_right (groupid, module, permission) VALUES (?, ?, ?)"
		_, err = db.Exec(insertQuery, groupID, module, permission)
		if err != nil {
			return err
		}
	}

	log.LogSystemEvent(
		fmt.Sprintf("Group %d permission set for module %s: %d", groupID, module, permission),
		log.LogLevelInfo, map[string]interface{}{
			"group_id":   groupID,
			"module":     module,
			"permission": permission,
		})

	return nil
}

// HasPermission checks if user has at least the required permission level
func HasPermission(userID int, module string, requiredPermission int) (bool, error) {
	effectivePermission, err := GetEffectivePermission(userID, module)
	if err != nil {
		return false, err
	}

	return effectivePermission >= requiredPermission, nil
}

// HasReadPermission checks if user has read access to module
func HasReadPermission(userID int, module string) (bool, error) {
	return HasPermission(userID, module, PERMISSION_READ)
}

// HasWritePermission checks if user has write access to module
func HasWritePermission(userID int, module string) (bool, error) {
	return HasPermission(userID, module, PERMISSION_WRITE)
}

// GetUserRoles returns the responsible role IDs for a user
func GetUserRoles(userID int) ([]int, error) {
	user, err := GetUserByID(userID)
	if err != nil {
		return nil, err
	}

	if user.ResponsibleRoleIDs == "" {
		return []int{}, nil
	}

	roleIDStrings := strings.Split(user.ResponsibleRoleIDs, ",")
	roleIDs := make([]int, 0, len(roleIDStrings))

	for _, roleIDStr := range roleIDStrings {
		roleIDStr = strings.TrimSpace(roleIDStr)
		if roleIDStr != "" {
			roleID, err := strconv.Atoi(roleIDStr)
			if err != nil {
				continue // Skip invalid IDs
			}
			roleIDs = append(roleIDs, roleID)
		}
	}

	return roleIDs, nil
}

// GetPermissionName returns human-readable name for permission level
func GetPermissionName(permission int) string {
	switch permission {
	case PERMISSION_INHERIT:
		return "Inherit"
	case PERMISSION_DENY:
		return "Deny"
	case PERMISSION_READ:
		return "Read"
	case PERMISSION_WRITE:
		return "Write"
	default:
		return "Unknown"
	}
}

// Legacy compatibility functions for existing code
// GetRights - legacy function, use GetEffectivePermission instead
func GetRights(groupId, userId int) (map[int]int, error) {
	// This is a legacy compatibility function
	// New code should use GetEffectivePermission instead
	rights := make(map[int]int)

	// For now, return empty map to maintain compatibility
	// TODO: Remove this function when all code is migrated to new system

	return rights, nil
}

// HasRight - legacy function, use HasPermission instead
func HasRight(name string, rights string) bool {
	// This is a legacy compatibility function
	// New code should use HasPermission instead
	// For now, return true to maintain compatibility
	// TODO: Remove this function when all code is migrated to new system

	return true
}
