package products

import (
	"net/http"

	"tls-rest/go/app"
	"tls-rest/go/engine/controllers/db/cache"
	"tls-rest/go/engine/controllers/db/pgdb"
	. "tls-rest/go/engine/controllers/field"
	"tls-rest/go/engine/controllers/functions"
	. "tls-rest/go/engine/controllers/module"
	"tls-rest/go/engine/controllers/request"
	"tls-rest/go/modules/payments"

	"github.com/gorilla/mux"
)

type Products struct {
	*ModuleAbstract[interface{}]
}

func (m *Products) fieldset() []Field {
	return []Field{
		NewField("name", TYPE_STRING, true).WithLabel("Name").WithValidation("minLength", 1).WithOption("width", "600px"),
		NewField("description", TYPE_MARKDOWN, false).WithLabel("Description").NonSortable(),
		NewField("price", TYPE_MONEY, true).WithLabel("Price").WithValidation("min", 0),
		NewField("currency", TYPE_STRING, true).WithLabel("Currency").WithDefault("USD").WithValidation("maxLength", 3),
		NewField("image", TYPE_IMAGE, false).WithLabel("Image").WithOption("folderTemplate", "products/{name}").NonSortable().NonSearchable(),
		NewField("active", TYPE_CHECKBOX, false).WithLabel("Active").WithDescription("Inactive products can't be bought").WithDefault(true),
	}
}

// pay handles POST /api/products/{id}/pay {platform_id, quantity}: charges one product through the chosen platform instance.
func pay(w http.ResponseWriter, r *http.Request) {
	s := cache.SessionFromContext(r.Context())
	if s == nil || s.UserID <= 0 {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	rq := request.From(r)
	qty := rq.Int("quantity")
	if qty <= 0 {
		qty = 1
	}
	if qty > payments.MaxQuantity {
		functions.JSONError(w, http.StatusBadRequest, "quantity too large")
		return
	}

	db, err := pgdb.GetInstanceCtx(r.Context())
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "database unavailable")
		return
	}
	level := s.AccessLevel
	if s.IsAdmin {
		level = 1 << 30
	}
	row, err := db.GetOne("SELECT id, name, price, currency FROM products WHERE id = $1 AND active = true AND access <= $2", mux.Vars(r)["id"], level)
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if row == nil {
		functions.JSONError(w, http.StatusNotFound, "product not found")
		return
	}

	out, err := payments.Checkout(r.Context(), s.UserID, int64(rq.Int("platform_id")), []payments.Item{{
		ProductID: functions.Coerce[int64](row["id"]),
		Name:      functions.Coerce[string](row["name"]),
		Price:     functions.Coerce[float64](row["price"]),
		Quantity:  qty,
		Currency:  functions.Coerce[string](row["currency"]),
	}})
	if err != nil {
		functions.JSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	functions.WriteJSON(w, http.StatusOK, out)
}

func New() *Products {
	m := &Products{
		ModuleAbstract: &ModuleAbstract[interface{}]{
			ID:      "products",
			Name:    "Products",
			Icon:    "config",
			Submenu: "shop",
			// Public catalog: everyone may browse; writes need an explicit grant.
			DefaultPermission:    PERMISSION_READ,
			DefaultPermissionSet: true,
			Rights:               make(map[int]int),
			CustomRoutes: []CustomRoute{
				{Path: "/api/products/{id}/pay", Methods: []string{http.MethodPost}, Handler: pay, Absolute: true},
			},
		},
	}
	m.ModuleAbstract.Fields = m.fieldset()
	return m
}

var Module = New()

func init() { app.RegisterModule(Module, "products") }
