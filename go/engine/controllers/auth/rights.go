package auth

import (
	"tls-rest/go/engine/controllers/module"
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

// ModuleDefaults returns the registry of module default permissions. There is a
// single source of truth: the engine populates it as each module initialises
// (module.RegisterModuleDefaultPermission). Earlier there was a second, empty
// copy in this package, which silently disabled the whole rights path — this
// accessor removes that duplication by reading the engine's map directly.
func ModuleDefaults() map[string]int {
	return module.ModuleDefaultPermissions
}

// IsAdmin reports whether the user belongs to an administrator group. It is a
// convenience wrapper around ResolveIsAdmin for callers that only have a user
// id; the request path resolves admin status once onto the session.
func IsAdmin(userID int) bool {
	return ResolveIsAdmin(userID)
}
