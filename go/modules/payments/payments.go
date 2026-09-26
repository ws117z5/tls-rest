package payments

import (
	"tls-rest/go/app"
	. "tls-rest/go/engine/controllers/field"
	. "tls-rest/go/engine/controllers/module"
)

type Payments struct {
	*ModuleAbstract[interface{}]
}

var statusOptions = func() []map[string]interface{} {
	return []map[string]interface{}{
		{"value": StatusPending, "name": "Pending"},
		{"value": StatusPaid, "name": "Paid"},
		{"value": StatusFailed, "name": "Failed"},
		{"value": StatusCancelled, "name": "Cancelled"},
		{"value": StatusRefunded, "name": "Refunded"},
	}
}

func (m *Payments) fieldset() []Field {
	return []Field{
		NewField("user_id", TYPE_INT, true).WithLabel("User").WithLink("users").AsReadOnly(),
		NewField("platform_id", TYPE_INT, true).WithLabel("Platform").WithLink("payment_platforms").AsReadOnly(),
		NewField("amount", TYPE_MONEY, true).WithLabel("Amount").AsReadOnly(),
		NewField("currency", TYPE_STRING, true).WithLabel("Currency").AsReadOnly(),
		NewField("status", TYPE_SELECT, true).WithLabel("Status").WithDefault(StatusPending).WithOptions(statusOptions),
		NewField("items", TYPE_JSON, false).WithLabel("Items").WithDescription("Snapshot of what was bought: product, unit price, quantity").AsReadOnly().NonSortable().NonSearchable(),
		NewField("external_id", TYPE_STRING, false).WithLabel("Platform reference").AsReadOnly(),
		NewField("details", TYPE_JSON, false).WithLabel("Platform response").AsReadOnly().AsAdminOnly().NonSortable().NonSearchable(),
	}
}

func New() *Payments {
	m := &Payments{
		ModuleAbstract: &ModuleAbstract[interface{}]{
			ID:      "payments",
			Name:    "Payments",
			Icon:    "config",
			Submenu: "shop",
			// Rows are created only by Checkout; non-admins see just their own (OwnerScoped) and need an explicit list/view grant.
			OwnerScoped:          true,
			DefaultPermission:    PERMISSION_DENY,
			DefaultPermissionSet: true,
			Rights:               make(map[int]int),
		},
	}
	m.ModuleAbstract.Fields = m.fieldset()
	return m
}

var Module = New()

func init() { app.RegisterModule(Module, "payments") }
