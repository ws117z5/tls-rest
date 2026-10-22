// Package external_logs stores diagnostic/crash log entries submitted by the
// external client apps (words-android, papers-android) over their own
// session token — a client-side "send me your crash logs" sink, distinct
// from go/engine/modules/logs (the server's own internal event log).
package external_logs

import (
	"net/http"

	"tls-rest/go/app"
	. "tls-rest/go/engine/controllers/field"
	. "tls-rest/go/engine/controllers/module"
)

func (m *ExternalLogs) fieldset() []Field {
	return []Field{
		NewField("app", TYPE_STRING, true).
			WithLabel("App").
			WithDescription("Client app identifier, e.g. \"words\" or \"papers\"").
			WithValidation("maxLength", 20),

		NewField("level", TYPE_STRING, true).
			WithLabel("Level").
			WithDescription("debug | info | warn | error").
			WithValidation("maxLength", 10),

		NewField("message", TYPE_TEXT, true).
			WithLabel("Message"),

		NewField("stack_trace", TYPE_TEXT, false).
			WithLabel("Stack trace").
			NonSearchable(),

		NewField("app_version", TYPE_STRING, false).
			WithLabel("App version").
			WithValidation("maxLength", 20),

		NewField("device_info", TYPE_STRING, false).
			WithLabel("Device").
			WithDescription("e.g. \"Android 14, Pixel 7\"").
			WithValidation("maxLength", 200),
	}
}

// filters: GET /external_logs?app=&level=&message=&from=&to=
func (m *ExternalLogs) filters() *Filedset {
	return NewFieldset(
		NewFilter("app", TYPE_STRING).WithLabel("App").Equals(),
		NewFilter("level", TYPE_STRING).WithLabel("Level").Equals(),
		NewFilter("message", TYPE_STRING).WithLabel("Message").Contains(),
		NewFilter("from", TYPE_DATE).WithLabel("From").WithSQL("created").GreaterOrEqual(),
		NewFilter("to", TYPE_DATE).WithLabel("To").WithSQL("created").LessOrEqual(),
	)
}

type ExternalLogs struct {
	*ModuleAbstract[interface{}]
}

func noEdit(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "external_logs entries are immutable", http.StatusMethodNotAllowed)
}

func NewExternalLogs() *ExternalLogs {
	m := &ExternalLogs{
		ModuleAbstract: &ModuleAbstract[interface{}]{
			ID:          "external_logs",
			Name:        "External Logs",
			Submenu:     "logs",
			Description: "Diagnostic/crash logs submitted by the client apps",

			// Each user only sees/creates their own logs; no group has access
			// until granted (init/sql/2026.10.16.sql grants the Users group
			// list/view/create — no edit, no delete).
			OwnerScoped:          true,
			DefaultPermission:    PERMISSION_DENY,
			DefaultPermissionSet: true,
			Rights:               make(map[int]int),

			// Append-only: a submitted log entry is never revised. Delete
			// isn't granted to the Users group either (see migration); admins
			// keep normal delete access for manual retention cleanup.
			Overrides: HandlerOverrides{Edit: noEdit},
		},
	}

	m.ModuleAbstract.Fields = m.fieldset()
	m.ModuleAbstract.Filters = m.filters()

	// "/config" is a literal path, registered ahead of the module's generic
	// "/{id}" view route so it wins the match (same convention words.go's
	// "/stats" documents).
	m.ModuleAbstract.CustomRoutes = []CustomRoute{
		{Path: "/config", Methods: []string{"GET"}, Handler: m.configHandler},
	}

	return m
}

var Module *ExternalLogs

func init() {
	Module = NewExternalLogs()
	app.RegisterModule(Module, "external_logs")
}
