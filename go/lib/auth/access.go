package auth

// Mode is one operation a user may be permitted to perform on a module. Rights
// are stored as an explicit per-mode bitmask per module (resolved from the
// user's group(s)); admin is allowed every mode.
type Mode = int

const (
	MODE_LIST   Mode = 1 << iota // 1  browse records
	MODE_VIEW                    // 2  read a single record
	MODE_CREATE                  // 4  create a record
	MODE_EDIT                    // 8  update a record
	MODE_DELETE                  // 16 delete a record
)

// MODE_ALL is every mode — the effective rights of an administrator.
const MODE_ALL = MODE_LIST | MODE_VIEW | MODE_CREATE | MODE_EDIT | MODE_DELETE

// ModuleModeRights maps moduleID -> allowed-mode bitmask for a single user,
// already resolved from their group memberships.
type ModuleModeRights map[string]int

// AllowedModes returns the mode bitmask a user has for a module (MODE_ALL for
// admins).
func AllowedModes(rights ModuleModeRights, module string, isAdmin bool) int {
	if isAdmin {
		return MODE_ALL
	}
	return rights[module]
}

// HasMode reports whether the user may perform mode on module.
func HasMode(rights ModuleModeRights, module string, mode Mode, isAdmin bool) bool {
	return AllowedModes(rights, module, isAdmin)&mode != 0
}

// modeName pairs a bit with its stable string name. This is the single source
// of the mode-bit -> name mapping; the API returns names (not the raw int) so
// clients never have to know this package's bit layout, which differs from the
// engine's field-visibility layout.
var modeNameTable = []struct {
	bit  Mode
	name string
}{
	{MODE_LIST, "list"},
	{MODE_VIEW, "view"},
	{MODE_CREATE, "create"},
	{MODE_EDIT, "edit"},
	{MODE_DELETE, "delete"},
}

// ModeNames converts an allowed-mode bitmask into the list of mode names set in
// it, in canonical order.
func ModeNames(mask int) []string {
	names := make([]string, 0, len(modeNameTable))
	for _, m := range modeNameTable {
		if mask&m.bit != 0 {
			names = append(names, m.name)
		}
	}
	return names
}

// AccessAll is the default access level of a record: visible to everyone.
const AccessAll = 0

// CanAccessRow reports whether a user at userLevel may see a record whose
// required access level is rowLevel. Higher level == more privileged; admins
// see everything, and lower-level users cannot see higher-level records.
func CanAccessRow(userLevel, rowLevel int, isAdmin bool) bool {
	if isAdmin {
		return true
	}
	return userLevel >= rowLevel
}
