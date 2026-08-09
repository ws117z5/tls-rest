package auth

import (
	"fmt"

	"tls-rest/go/lib/log"
)

// Permission values used for a module's *default* permission (the baseline
// before any group/user grant). The live, per-request rights model is the
// per-mode bitmask in access.go, resolved in resolve.go; these constants only
// seed module defaults via defaultModesFor.
const (
	PERMISSION_INHERIT = -1 // Inherit from group/default
	PERMISSION_DENY    = 0  // No access
	PERMISSION_READ    = 1  // Read-only (list/view)
	PERMISSION_WRITE   = 2  // Full access
)

// ModuleDefaultPermissions is the registry of module default permissions,
// populated by the engine as each module initialises.
var ModuleDefaultPermissions = make(map[string]int)

// RegisterModuleDefaultPermission records a module's default permission.
func RegisterModuleDefaultPermission(module string, defaultPermission int) {
	ModuleDefaultPermissions[module] = defaultPermission
	log.LogSystemEvent(
		fmt.Sprintf("Default permission %d registered for module %s", defaultPermission, module),
		log.LogLevelInfo, map[string]interface{}{
			"module":             module,
			"default_permission": defaultPermission,
		})
}

// IsAdmin reports whether the user belongs to an administrator group. It is a
// convenience wrapper around ResolveIsAdmin for callers that only have a user
// id; the request path resolves admin status once onto the session.
func IsAdmin(userID int) bool {
	return ResolveIsAdmin(userID)
}
