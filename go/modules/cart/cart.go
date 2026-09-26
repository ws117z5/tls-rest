package cart

import (
	"context"
	"net/http"

	"tls-rest/go/app"
	"tls-rest/go/engine/controllers/db/cache"
	"tls-rest/go/engine/controllers/db/pgdb"
	. "tls-rest/go/engine/controllers/field"
	"tls-rest/go/engine/controllers/functions"
	. "tls-rest/go/engine/controllers/module"
	"tls-rest/go/engine/controllers/request"
	"tls-rest/go/modules/payments"
)

type Cart struct {
	*ModuleAbstract[interface{}]
}

func searchProducts(ctx context.Context, input string, _ map[string]interface{}) []AutoOption {
	db, err := pgdb.GetInstanceCtx(ctx)
	if err != nil {
		return []AutoOption{}
	}
	rows, err := db.GetAll("SELECT id, name FROM products WHERE active = true AND name ILIKE $1 ORDER BY name LIMIT 20", "%"+input+"%")
	if err != nil {
		return []AutoOption{}
	}
	out := make([]AutoOption, 0, len(rows))
	for _, r := range rows {
		out = append(out, AutoOption{Value: functions.Coerce[string](r["id"]), Label: functions.Coerce[string](r["name"])})
	}
	return out
}

func productName(ctx context.Context, id string, _ map[string]interface{}) (string, bool) {
	db, err := pgdb.GetInstanceCtx(ctx)
	if err != nil {
		return "", false
	}
	row, err := db.GetOne("SELECT name FROM products WHERE id = $1", id)
	if err != nil || row == nil {
		return "", false
	}
	return functions.Coerce[string](row["name"]), true
}

func (m *Cart) fieldset() []Field {
	return []Field{
		NewField("product_id", TYPE_INT, true).WithLabel("Product").WithAutocomplete(map[string]interface{}{
			"function": searchProducts,
			"view":     productName,
		}),
		NewField("quantity", TYPE_INT, true).WithLabel("Quantity").WithDefault(1).WithValidation("min", 1),
		NewField("product_name", TYPE_STRING, false).WithLabel("Product name").
			WithSQL("(SELECT name FROM products WHERE products.id = cart.product_id)").AsVirtual().NonSortable().NonSearchable(),
		NewField("unit_price", TYPE_MONEY, false).WithLabel("Unit price").
			WithSQL("(SELECT price FROM products WHERE products.id = cart.product_id)").AsVirtual().NonSortable(),
		NewField("subtotal", TYPE_MONEY, false).WithLabel("Subtotal").
			WithSQL("(SELECT price FROM products WHERE products.id = cart.product_id) * cart.quantity").AsVirtual().NonSortable(),
	}
}

func session(w http.ResponseWriter, r *http.Request) *cache.Session {
	s := cache.SessionFromContext(r.Context())
	if s == nil || s.UserID <= 0 {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return nil
	}
	return s
}

// add handles POST /api/cart/add {product_id, quantity}: puts a product in the caller's cart, or raises its quantity.
func add(w http.ResponseWriter, r *http.Request) {
	s := session(w, r)
	if s == nil {
		return
	}
	rq := request.From(r)
	productID := rq.Int("product_id")
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
	product, err := db.GetOne("SELECT id FROM products WHERE id = $1 AND active = true AND access <= $2", productID, accessLevel(s))
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if product == nil {
		functions.JSONError(w, http.StatusNotFound, "product not found")
		return
	}

	if err := putInCart(db, s.UserID, int64(productID), qty); err != nil {
		functions.JSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	functions.WriteJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

// accessLevel is the product access level the caller may buy at (admins: everything).
func accessLevel(s *cache.Session) int {
	if s.IsAdmin {
		return 1 << 30
	}
	return s.AccessLevel
}

// putInCart adds qty of a product to the user's cart in one statement, raising an existing line but never past MaxQuantity.
func putInCart(db *pgdb.Db, userID int, productID int64, qty int) error {
	_, err := db.Exec(`
		INSERT INTO cart (product_id, quantity, created_by) VALUES ($1, LEAST($2, $4), $3)
		ON CONFLICT (created_by, product_id) DO UPDATE SET quantity = LEAST(cart.quantity + EXCLUDED.quantity, $4)`,
		productID, qty, userID, payments.MaxQuantity)
	return err
}

// checkout handles POST /api/cart/checkout {platform_id}: pays for the whole cart, emptying it unless the charge failed.
func checkout(w http.ResponseWriter, r *http.Request) {
	s := session(w, r)
	if s == nil {
		return
	}
	db, err := pgdb.GetInstanceCtx(r.Context())
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "database unavailable")
		return
	}
	// Taking the lines out of the cart is one atomic statement, so two concurrent checkouts can't both charge the same lines.
	taken, err := db.GetAll("DELETE FROM cart WHERE created_by = $1 RETURNING product_id, quantity", s.UserID)
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(taken) == 0 {
		functions.JSONError(w, http.StatusBadRequest, "cart is empty")
		return
	}
	restore := func(rows []map[string]interface{}) {
		for _, row := range rows {
			_ = putInCart(db, s.UserID, functions.Coerce[int64](row["product_id"]), functions.Int(row["quantity"]))
		}
	}

	ids := make([]int64, 0, len(taken))
	for _, row := range taken {
		ids = append(ids, functions.Coerce[int64](row["product_id"]))
	}
	products, err := db.GetAll("SELECT id, name, price, currency FROM products WHERE id = ANY($1) AND active = true AND access <= $2", ids, accessLevel(s))
	if err != nil {
		restore(taken)
		functions.JSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	byID := make(map[int64]map[string]interface{}, len(products))
	for _, p := range products {
		byID[functions.Coerce[int64](p["id"])] = p
	}

	// Lines whose product is no longer buyable stay in the cart.
	var buyable, leftover []map[string]interface{}
	items := make([]payments.Item, 0, len(taken))
	for _, row := range taken {
		p := byID[functions.Coerce[int64](row["product_id"])]
		if p == nil {
			leftover = append(leftover, row)
			continue
		}
		buyable = append(buyable, row)
		items = append(items, payments.Item{
			ProductID: functions.Coerce[int64](p["id"]),
			Name:      functions.Coerce[string](p["name"]),
			Price:     functions.Coerce[float64](p["price"]),
			Quantity:  functions.Int(row["quantity"]),
			Currency:  functions.Coerce[string](p["currency"]),
		})
	}
	restore(leftover)

	out, err := payments.Checkout(r.Context(), s.UserID, int64(request.From(r).Int("platform_id")), items)
	if err != nil {
		restore(buyable)
		functions.JSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	functions.WriteJSON(w, http.StatusOK, out)
}

func New() *Cart {
	m := &Cart{
		ModuleAbstract: &ModuleAbstract[interface{}]{
			ID:      "cart",
			Name:    "Cart",
			Icon:    "config",
			Submenu: "shop",
			// Each user sees only their own rows; they need an explicit grant since guests have no owner to scope by.
			OwnerScoped:          true,
			DefaultPermission:    PERMISSION_DENY,
			DefaultPermissionSet: true,
			Rights:               make(map[int]int),
			CustomRoutes: []CustomRoute{
				{Path: "/api/cart/add", Methods: []string{http.MethodPost}, Handler: add, Absolute: true},
				{Path: "/api/cart/checkout", Methods: []string{http.MethodPost}, Handler: checkout, Absolute: true},
			},
		},
	}
	m.ModuleAbstract.Fields = m.fieldset()
	return m
}

var Module = New()

func init() { app.RegisterModule(Module, "cart") }
