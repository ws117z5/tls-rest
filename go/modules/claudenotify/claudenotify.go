// Package claudenotify backs a tiny feed Claude Code's own "Stop" hook posts
// to (any session, any project, see ~/.claude/settings.json on this machine)
// so the companion Android app can show "<session> done" in real time.
package claudenotify

import (
	"net/http"

	"tls-rest/go/app"
	. "tls-rest/go/engine/controllers/field"
	"tls-rest/go/engine/controllers/functions"
	. "tls-rest/go/engine/controllers/module"
)

type ClaudeNotify struct {
	*ModuleAbstract[interface{}]
}

func (m *ClaudeNotify) fieldset() []Field {
	return []Field{
		NewField("session", TYPE_STRING, true).
			WithLabel("Session"),
		NewField("message", TYPE_STRING, false).
			WithLabel("Message").
			WithDefault("Task done"),
	}
}

// pingOwner is an AfterFieldset hook: wakes the creating user's SSE
// subscribers right after a notification is inserted. created_by is already
// stamped onto data by this point (base_controller.Create runs it before
// AfterFieldset).
func pingOwner(_ *http.Request, data map[string]interface{}) (map[string]interface{}, error) {
	if uid := functions.Int(data["created_by"]); uid > 0 {
		hub.notify(uid)
	}
	return data, nil
}

func NewClaudeNotify() *ClaudeNotify {
	m := &ClaudeNotify{
		ModuleAbstract: &ModuleAbstract[interface{}]{
			ID:      "claude_notifications",
			Name:    "Claude Notifications",
			Icon:    "config",
			Submenu: "dev",
			// Each user sees only their own notifications; they need an explicit
			// grant since guests have no owner to scope by (see init/sql/2026.10.13.sql).
			OwnerScoped:          true,
			DefaultPermission:    PERMISSION_DENY,
			DefaultPermissionSet: true,
			Rights:               make(map[int]int),
			CustomRoutes: []CustomRoute{
				{Path: "/api/claude-notifications/stream", Methods: []string{http.MethodGet}, Handler: handleStream, Absolute: true},
			},
		},
	}
	m.ModuleAbstract.Fields = m.fieldset()
	m.ModuleAbstract.AfterFieldset = pingOwner
	return m
}

func init() {
	app.RegisterModule(NewClaudeNotify(), "claude_notifications")
}
