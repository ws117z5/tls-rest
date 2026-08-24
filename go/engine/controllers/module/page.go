package module

import (
	"encoding/json"
	"net/http"

	"tls-rest/go/engine/controllers/db/cache"
	"tls-rest/go/engine/controllers/field"

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
// gated by rights. It is the backend twin of the frontend Fieldset page. Unlike a
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
	Fields   []field.Field
	Editable bool // when true and Save != nil, a PUT handler is registered

	RequiresAuth  bool
	RequiresAdmin bool
	Order         int    // menu ordering (lower first)
	Submenu       string // groups this page under a named submenu; empty = top level
	Icon          string // menu icon (URL, e.g. /image/<uuid> or a static path)

	// Load returns the single record for this page (given the session, so a page
	// can be "the current user", "this org", ...). Save persists an update.
	Load func(s *cache.Session) (map[string]interface{}, error)
	Save func(s *cache.Session, data map[string]interface{}) error

	// Custom routes with their own handlers (non-fieldset pages).
	Routes []PageRoute
}

// Initialize registers the page's routes via the shared registrar seam.
func (p *PageAbstract) Initialize() {
	// Publish menu metadata so /api/pages can list it (session-filtered),
	// making the pages menu backend-driven like modules.
	registerPageMenu(PageMenuMeta{
		ID: p.ID, Name: p.Name,
		RequiresAuth: p.RequiresAuth, RequiresAdmin: p.RequiresAdmin, Order: p.Order,
		Submenu: p.Submenu, Icon: p.Icon,
	})

	// Advertise this page's data endpoints (fieldset endpoint + custom routes)
	// so the middleware recognizes them without a hardcoded list.
	if p.Endpoint != "" {
		RegisterEndpointPrefix(p.Endpoint)
	}
	for _, rt := range p.Routes {
		RegisterEndpointPrefix(rt.Path)
	}

	AddRouteRegistrar(func(router *mux.Router) {
		for _, rt := range p.Routes {
			// Enforce the page's RequiresAuth/RequiresAdmin on custom routes too,
			// not just the fieldset endpoint — otherwise the flags would be
			// decorative for pages whose real work is a custom route.
			route := router.HandleFunc(rt.Path, p.guard(rt.Handler))
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

// guard wraps a custom-route handler with this page's RequiresAuth/RequiresAdmin
// enforcement, matching the fieldset endpoint's checks: an unauthenticated
// caller on an auth-required page gets 401, a non-admin on an admin-required
// page gets 403. When the page requires neither, the handler is returned
// unwrapped (zero overhead).
func (p *PageAbstract) guard(next http.HandlerFunc) http.HandlerFunc {
	if !p.RequiresAuth && !p.RequiresAdmin {
		return next
	}
	return func(w http.ResponseWriter, r *http.Request) {
		s := cache.SessionFromContext(r.Context())
		if p.RequiresAuth && (s == nil || s.UserID <= 0) {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		if p.RequiresAdmin && (s == nil || !s.IsAdmin) {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}
		next(w, r)
	}
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
	fields := make([]field.Field, 0, len(p.Fields))
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
