// Package reports is the admin-only "block and report" queue — see
// go/engine/modules/chats' handleReportUser. An open report against a user
// blocks them from sending new friend requests (go/engine/modules/chats'
// hasOpenReport) until an admin flips a row's status to 'cleared' here.
package reports

import (
	"tls-rest/go/app"
	. "tls-rest/go/engine/controllers/field"
	. "tls-rest/go/engine/controllers/module"
)

type Reports struct {
	*ModuleAbstract[interface{}]
}

var statusOptions = func() []map[string]interface{} {
	return []map[string]interface{}{
		{"value": "open", "name": "Open"},
		{"value": "cleared", "name": "Cleared"},
	}
}

func (m *Reports) fieldset() []Field {
	return []Field{
		NewField("reporter_id", TYPE_INT, true).WithLabel("Reporter").WithLink("users").AsReadOnly(),
		NewField("reported_id", TYPE_INT, true).WithLabel("Reported user").WithLink("users").AsReadOnly(),
		NewField("message", TYPE_TEXT, false).WithLabel("Message").AsReadOnly(),
		NewField("status", TYPE_SELECT, true).WithLabel("Status").WithDefault("open").WithOptions(statusOptions),
	}
}

func New() *Reports {
	m := &Reports{
		ModuleAbstract: &ModuleAbstract[interface{}]{
			ID:      "reports",
			Name:    "Reported users",
			Icon:    "statistics",
			Submenu: "users",
			// No non-admin can see any report, even their own — admin-only by
			// construction (is_admin bypasses rights entirely; everyone else
			// gets no user_group_rights grant for this module at all).
			DefaultPermission:    PERMISSION_DENY,
			DefaultPermissionSet: true,
			Rights:               make(map[int]int),
		},
	}
	m.ModuleAbstract.Fields = m.fieldset()
	return m
}

var Module = New()

func init() { app.RegisterModule(Module, "reports") }
