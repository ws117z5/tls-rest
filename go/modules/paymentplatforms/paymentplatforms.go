package paymentplatforms

import (
	"net/http"

	"tls-rest/go/app"
	"tls-rest/go/engine/controllers/db/cache"
	"tls-rest/go/engine/controllers/db/pgdb"
	. "tls-rest/go/engine/controllers/field"
	"tls-rest/go/engine/controllers/functions"
	. "tls-rest/go/engine/controllers/module"
)

type PaymentPlatforms struct {
	*ModuleAbstract[interface{}]
}

func (m *PaymentPlatforms) fieldset() []Field {
	return []Field{
		NewField("name", TYPE_STRING, true).WithLabel("Name").WithValidation("minLength", 1),
		NewField("provider", TYPE_SELECT, true).WithLabel("Provider").WithOptions(ProviderOptions),
		NewField("enabled", TYPE_CHECKBOX, false).WithLabel("Enabled").WithDefault(true),
		NewField("sandbox", TYPE_CHECKBOX, false).WithLabel("Sandbox").WithDefault(true),
		NewField("api_key", TYPE_PASSWORD, false).WithLabel("API key").WithDescription("Write-only; never shown after saving"),
		NewField("config", TYPE_JSON, false).WithLabel("Config").WithDescription("Provider settings, e.g. {\"instructions\": \"...\"} for manual").NonSortable().NonSearchable(),
	}
}

// available lists enabled platforms (id, name, provider only, no credentials) for the payer's platform picker.
func available(w http.ResponseWriter, r *http.Request) {
	if s := cache.SessionFromContext(r.Context()); s == nil || s.UserID <= 0 {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	db, err := pgdb.GetInstanceCtx(r.Context())
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "database unavailable")
		return
	}
	rows, err := db.GetAll("SELECT id, name, provider FROM payment_platforms WHERE enabled = true ORDER BY name")
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	functions.WriteJSON(w, http.StatusOK, rows)
}

func New() *PaymentPlatforms {
	m := &PaymentPlatforms{
		ModuleAbstract: &ModuleAbstract[interface{}]{
			ID:                   "payment_platforms",
			Name:                 "Payment Platforms",
			Icon:                 "config",
			Submenu:              "shop",
			DefaultPermission:    PERMISSION_DENY,
			DefaultPermissionSet: true,
			Rights:               make(map[int]int),
			CustomRoutes: []CustomRoute{
				{Path: "/api/payment_platforms/available", Methods: []string{http.MethodGet}, Handler: available, Absolute: true},
			},
		},
	}
	m.ModuleAbstract.Fields = m.fieldset()
	return m
}

var Module = New()

func init() { app.RegisterModule(Module, "payment_platforms") }
