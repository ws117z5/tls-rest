package accesslog

import (
	"net/http"
	. "tls-rest/go/engine/controllers/field"
	"tls-rest/go/engine/controllers/module"
)

// fieldset maps the access_log columns shown in the admin UI. Field names match
// the table columns exactly (the engine builds SELECT/WHERE from them). All are
// read-only; the module itself is read-only.
func (m *AccessLog) fieldset() []Field {
	return []Field{
		NewField("ts", TYPE_DATE_TIME, false).WithLabel("Time").AsReadOnly(),
		NewField("method", TYPE_STRING, false).WithLabel("Method").AsReadOnly(),
		NewField("path", TYPE_STRING, false).WithLabel("Path").AsReadOnly(),
		NewField("status", TYPE_INT, false).WithLabel("Status").AsReadOnly(),
		NewField("duration_ms", TYPE_FLOAT, false).WithLabel("Duration (ms)").AsReadOnly(),
		NewField("user_id", TYPE_INT, false).WithLabel("User").AsReadOnly(),
		NewField("ip", TYPE_STRING, false).WithLabel("IP").AsReadOnly(),
		NewField("user_agent", TYPE_STRING, false).WithLabel("User agent").AsReadOnly().NonSortable(),
		NewField("module", TYPE_STRING, false).WithLabel("Module").AsReadOnly(),
		NewField("action", TYPE_STRING, false).WithLabel("Action").AsReadOnly(),
		NewField("blocked", TYPE_CHECKBOX, false).WithLabel("Blocked").AsReadOnly(),
		NewField("denied_reason", TYPE_STRING, false).WithLabel("Denied reason").AsReadOnly(),
	}
}

// filters declares list-mode filters:
// GET /access_log?status=&method=&ip=&path=&blocked=&from=&to=
func (m *AccessLog) filters() *Filedset {
	return NewFieldset(
		NewFilter("status", TYPE_INT).WithLabel("Status").Equals(),
		NewFilter("method", TYPE_STRING).WithLabel("Method").Equals(),
		NewFilter("ip", TYPE_STRING).WithLabel("IP").Contains(),
		NewFilter("path", TYPE_STRING).WithLabel("Path").Contains(),
		NewFilter("blocked", TYPE_CHECKBOX).WithLabel("Blocked only").Equals(),
		NewFilter("from", TYPE_DATE).WithLabel("From").WithSQL("ts").GreaterOrEqual(),
		NewFilter("to", TYPE_DATE).WithLabel("To").WithSQL("ts").LessOrEqual(),
	)
}

// AccessLog is a read-only, admin-only engine module over the access_log table
// (written by go/lib/accesslog). It demonstrates OmitSystemFields: the table is a
// purpose-built log schema with only an `id` from the standard set, so uuid /
// created / updated / created_by / access are dropped.
type AccessLog struct {
	*module.ModuleAbstract[interface{}]
}

func readOnly(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "access_log is read-only", http.StatusMethodNotAllowed)
}

// NewAccessLog builds the module instance.
func NewAccessLog() *AccessLog {
	m := &AccessLog{
		ModuleAbstract: &module.ModuleAbstract[interface{}]{
			ID:       "access_log",
			Name:     "Access Log",
			Submenu:  "engine",
			ReadOnly: true,
			Rights:   make(map[int]int),
			// Admin-only: DENY as the default means non-admins never reach it;
			// admins bypass. DefaultPermissionSet keeps the 0 from being promoted
			// to READ.
			DefaultPermission:    module.PERMISSION_DENY,
			DefaultPermissionSet: true,
			// Drop every standard field except id (this table has none of them).
			OmitSystemFields: []string{"uuid", "created", "updated", "created_by", "access"},
			// Read-only: writes are refused; only List/View are meaningful.
			Overrides: module.HandlerOverrides{
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

func Init() {
	NewAccessLog().Initialize("access_log")
}
