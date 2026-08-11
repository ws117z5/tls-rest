package module

import (
	"encoding/json"
	"net/http"

	"tls-rest/go/lib/db/cache"

	"github.com/gorilla/mux"
)

// --- Shared route-registrar seam ---------------------------------------------
//
// Routes can't be added at package init() time because the mux router doesn't
// exist yet. Packages therefore queue a registrar here at init(); the router
// builder flushes them once the router exists (FlushRouteRegistrars). This is
// the same deferred idea the module system already uses (SetGlobalRouter), made
// available to pages and features so they can own their own routes instead of
// route.go hardcoding them.

var routeRegistrars []func(*mux.Router)

// AddRouteRegistrar queues a route registration. If the router already exists
// (registration after startup), it is applied immediately.
func AddRouteRegistrar(fn func(*mux.Router)) {
	if fn == nil {
		return
	}
	routeRegistrars = append(routeRegistrars, fn)
	if GlobalRouter != nil {
		fn(GlobalRouter)
	}
}

// FlushRouteRegistrars applies all queued registrars to the router. Called once
// by the router builder after the router is created.
func FlushRouteRegistrars(router *mux.Router) {
	for _, fn := range routeRegistrars {
		fn(router)
	}
}

// --- PageAbstract ------------------------------------------------------------
//
// A Page is a standalone, non-module screen: one representation, no modes, fields
// gated by rights. It is the backend twin of the frontend FieldsetPage. Unlike a
// module it has no CRUD/list/table and creates no DB table — it reads/writes a
// single record through Load/Save (e.g. profile == the current user's row).
//
// A page can be:
//   - a fieldset page: set Endpoint + Fields + Load (+ Save/Editable). The base
//     supplies generic GET ({Data, Fieldset}) and PUT handlers, rights-filtered.
//   - a custom page: set Routes with your own handlers (e.g. login). Both may be
//     combined.

type PageRoute struct {
	Path    string
	Methods []string
	Handler http.HandlerFunc
}

type PageAbstract struct {
	ID       string // "profile"
	Name     string // "Profile"
	Endpoint string // "/api/profile" (fieldset page); empty for custom-only pages
	Fields   []Field
	Editable bool // when true and Save != nil, a PUT handler is registered

	RequiresAuth  bool
	RequiresAdmin bool

	// Load returns the single record for this page (given the session, so a page
	// can be "the current user", "this org", ...). Save persists an update.
	Load func(s *cache.Session) (map[string]interface{}, error)
	Save func(s *cache.Session, data map[string]interface{}) error

	// Custom routes with their own handlers (non-fieldset pages).
	Routes []PageRoute
}

// Initialize registers the page's routes via the shared registrar seam.
func (p *PageAbstract) Initialize() {
	AddRouteRegistrar(func(router *mux.Router) {
		for _, rt := range p.Routes {
			route := router.HandleFunc(rt.Path, rt.Handler)
			if len(rt.Methods) > 0 {
				route.Methods(rt.Methods...)
			}
		}

		if p.Endpoint != "" && p.Load != nil {
			router.HandleFunc(p.Endpoint, p.handleGet).Methods("GET")
			if p.Editable && p.Save != nil {
				router.HandleFunc(p.Endpoint, p.handlePut).Methods("PUT")
			}
		}
	})
}

func (p *PageAbstract) authorized(s *cache.Session) bool {
	if p.RequiresAuth && (s == nil || s.UserID <= 0) {
		return false
	}
	if p.RequiresAdmin && (s == nil || !s.IsAdmin) {
		return false
	}
	return true
}

// handleGet returns { Data, Fieldset:{fields} } with fields filtered to what the
// requesting user may see (system fields admin-only, access-gated fields hidden).
func (p *PageAbstract) handleGet(w http.ResponseWriter, r *http.Request) {
	s := cache.SessionFromContext(r.Context())
	if !p.authorized(s) {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	data, err := p.Load(s)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	v := viewerFromRequest(r)
	fields := make([]Field, 0, len(p.Fields))
	for _, f := range p.Fields {
		if v.fieldVisibleInSchema(f) {
			fields = append(fields, f)
		}
	}

	writePageJSON(w, http.StatusOK, map[string]interface{}{
		"Data": data,
		"Fieldset": map[string]interface{}{
			"id":     p.ID,
			"name":   p.Name,
			"fields": fields,
		},
	})
}

// handlePut persists an update, accepting only fields the user may see and that
// aren't read-only (so a user can't write hidden/admin/access-gated fields).
func (p *PageAbstract) handlePut(w http.ResponseWriter, r *http.Request) {
	s := cache.SessionFromContext(r.Context())
	if !p.authorized(s) {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	v := viewerFromRequest(r)
	update := map[string]interface{}{}
	for _, f := range p.Fields {
		if f.ReadOnly || !v.fieldVisibleInSchema(f) {
			continue
		}
		if val, ok := body[f.Name]; ok {
			update[f.Name] = val
		}
	}

	if err := p.Save(s, update); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	p.handleGet(w, r) // return the fresh record
}

func writePageJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
