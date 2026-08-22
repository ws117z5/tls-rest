package accessrule

import (
	. "tls-rest/go/engine/controllers/field"
	. "tls-rest/go/engine/controllers/module"
)

// AccessRule is the admin-only engine module that manages IP/CIDR allow-deny
// rules in the access_rule table. The middleware enforces them (via
// go/lib/accesslog); the rule cache re-reads the table on a 30s TTL, so edits
// here take effect within ~30s without an explicit reload hook.
//
// Like access_log it drops the standard system fields the table doesn't have,
// keeping only id.
type AccessRule struct {
	*ModuleAbstract[interface{}]
}

func (m *AccessRule) fieldset() []Field {
	return []Field{
		NewField("cidr", TYPE_STRING, true).
			WithLabel("IP or CIDR").
			WithDescription("A single IP (203.0.113.7) or a range (203.0.113.0/24)").
			WithValidation("minLength", 3),

		NewField("action", TYPE_SELECT, true).
			WithLabel("Action").
			WithDescription("Whether matching clients are allowed or blocked").
			WithOption("options", []map[string]interface{}{
				{"value": "deny", "name": "Deny"},
				{"value": "allow", "name": "Allow"},
			}).
			WithDefault("deny"),

		NewField("priority", TYPE_INT, true).
			WithLabel("Priority").
			WithDescription("Lower is evaluated first; the first matching rule wins").
			WithDefault(100).
			WithValidation("min", 0),

		NewField("enabled", TYPE_CHECKBOX, false).
			WithLabel("Enabled").
			WithDefault(true),

		NewField("note", TYPE_STRING, false).
			WithLabel("Note").
			WithOption("width", "400px"),
	}
}

// filters: GET /access_rule?action=&enabled=&cidr=
func (m *AccessRule) filters() *Filedset {
	return NewFieldset(
		NewFilter("action", TYPE_STRING).WithLabel("Action").Equals(),
		NewFilter("enabled", TYPE_CHECKBOX).WithLabel("Enabled").Equals(),
		NewFilter("cidr", TYPE_STRING).WithLabel("IP/CIDR").Contains(),
	)
}

func NewAccessRule() *AccessRule {
	m := &AccessRule{
		ModuleAbstract: &ModuleAbstract[interface{}]{
			ID:                   "access_rule",
			Name:                 "Access Rules",
			Submenu:              "engine",
			Rights:               make(map[int]int),
			DefaultPermission:    PERMISSION_DENY, // admin-only
			DefaultPermissionSet: true,
			OmitSystemFields:     []string{"uuid", "created", "updated", "created_by", "access"},
		},
	}
	m.ModuleAbstract.Fields = m.fieldset()
	m.ModuleAbstract.Filters = m.filters()
	return m
}

func Init() {
	NewAccessRule().Initialize("access_rule")
}
