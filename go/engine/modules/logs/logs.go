package logs

import (
	"net/http"
	. "tls-rest/go/engine/controllers/field"
	. "tls-rest/go/engine/controllers/module"
)

// fieldset maps the `logs` columns. Field names must match the columns the DB
// sink writes (go/lib/logdb.Init); the engine auto-creates the table from this
// fieldset, so this is the single source of truth for the schema.
func (m *Logs) fieldset() []Field {
	return []Field{
		NewField("ts", TYPE_DATE_TIME, false).WithLabel("Time").AsReadOnly(),
		NewField("level", TYPE_STRING, false).WithLabel("Level").AsReadOnly(),
		NewField("type", TYPE_STRING, false).WithLabel("Type").AsReadOnly(),
		NewField("message", TYPE_STRING, false).WithLabel("Message").AsReadOnly(),
		NewField("module", TYPE_STRING, false).WithLabel("Module").AsReadOnly(),
		NewField("action", TYPE_STRING, false).WithLabel("Action").AsReadOnly(),
		NewField("user_id", TYPE_INT, false).WithLabel("User").AsReadOnly(),
		NewField("method", TYPE_STRING, false).WithLabel("Method").AsReadOnly(),
		NewField("request_url", TYPE_STRING, false).WithLabel("URL").AsReadOnly().NonSortable(),
		NewField("status_code", TYPE_INT, false).WithLabel("Status").AsReadOnly(),
		NewField("duration_ms", TYPE_FLOAT, false).WithLabel("Duration (ms)").AsReadOnly(),
		NewField("ip_address", TYPE_STRING, false).WithLabel("IP").AsReadOnly(),
		NewField("session_id", TYPE_STRING, false).WithLabel("Session").AsReadOnly().NonSortable(),
		NewField("event_id", TYPE_STRING, false).WithLabel("Event ID").AsReadOnly().NonSortable(),
		NewField("error", TYPE_STRING, false).WithLabel("Error").AsReadOnly(),
	}
}

// filters: GET /logs?level=&type=&module=&method=&message=&from=&to=
func (m *Logs) filters() *Filedset {
	return NewFieldset(
		NewFilter("level", TYPE_STRING).WithLabel("Level").Equals(),
		NewFilter("type", TYPE_STRING).WithLabel("Type").Equals(),
		NewFilter("module", TYPE_STRING).WithLabel("Module").Equals(),
		NewFilter("method", TYPE_STRING).WithLabel("Method").Equals(),
		NewFilter("message", TYPE_STRING).WithLabel("Message").Contains(),
		NewFilter("from", TYPE_DATE).WithLabel("From").WithSQL("ts").GreaterOrEqual(),
		NewFilter("to", TYPE_DATE).WithLabel("To").WithSQL("ts").LessOrEqual(),
	)
}

type Logs struct {
	*ModuleAbstract[interface{}]
}

func readOnly(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "logs is read-only", http.StatusMethodNotAllowed)
}

func NewLogs() *Logs {
	m := &Logs{
		ModuleAbstract: &ModuleAbstract[interface{}]{
			ID:          "logs",
			Name:        "Logs",
			Submenu:     "engine",
			ReadOnly:    true,
			Description: "Application event log",
			Rights:      make(map[int]int),
			// Admin-only: DENY default gates non-admins; admins bypass.
			DefaultPermission:    PERMISSION_DENY,
			DefaultPermissionSet: true,
			// The logs table has none of the standard system columns except id.
			OmitSystemFields: []string{"uuid", "created", "updated", "created_by", "access"},
			// Read-only: the event logger writes rows; the UI only browses them.
			Overrides: HandlerOverrides{
				Create: readOnly,
				Edit:   readOnly,
				Delete: readOnly,
			},
		},
	}
	m.ModuleAbstract.Fields = m.fieldset()
	m.ModuleAbstract.Filters = m.filters()
	return m
}

// Register wires this package into the engine (called from go/imports.go).
func Init() {
	NewLogs().Initialize("logs")
}
