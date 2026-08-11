package route

import (
	"net/http"
	"strings"

	"github.com/gorilla/mux"

	"tls-rest/go/controllers"
	"tls-rest/go/controllers/log"
	"tls-rest/go/lib/auth"

	// Import modules/pages/features to trigger their init() self-registration.
	_ "tls-rest/go/engine/features/images"
	_ "tls-rest/go/engine/modules/modulerights"
	_ "tls-rest/go/engine/modules/posts"
	_ "tls-rest/go/engine/modules/usergroups"
	_ "tls-rest/go/engine/modules/users"
	_ "tls-rest/go/engine/pages/login"
	_ "tls-rest/go/engine/pages/profile"
	_ "tls-rest/go/features/opencv"
	_ "tls-rest/go/features/papers"

	module "tls-rest/go/engine"
	middleware "tls-rest/go/lib/route/middlware"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var DefaultMethod = []string{"GET", "POST"}

type Route struct {
	Name      string
	Methods   []string
	Pattern   string
	Handler   http.HandlerFunc
	Satatic   bool // if true, serve static files
	Params    []string
	Subroutes []Route
}

var routes = []Route{
	{
		Name:    "Index",
		Methods: DefaultMethod,
		Pattern: "/",
		Handler: controllers.Index,
		Satatic: false,
	},
	{
		Name:    "Pages",
		Methods: DefaultMethod,
		Pattern: "/pages",
		Handler: controllers.Index,
		Satatic: false,
	},
	{
		Name:    "Log",
		Methods: []string{"POST"},
		Pattern: "/log",
		Handler: log.Log1,
		Satatic: false,
	},
	{
		Name:    "Login",
		Methods: DefaultMethod,
		Pattern: "/login",
		Handler: controllers.Index,
		Satatic: false,
	},
	{
		Name:    "Profile",
		Methods: DefaultMethod,
		Pattern: "/profile",
		Handler: controllers.Index,
		Satatic: false,
	},
	// papers now self-registers its routes (features/papers/routes.go).
	// Users routes are now automatically registered via module system
	// Posts routes are now automatically registered via module system
	// Static routes are now handled by RegisterStaticRoutes() function
	{
		Name:    "Metrics",
		Methods: DefaultMethod,
		Pattern: "/metrics",
		Handler: promhttp.Handler().ServeHTTP,
		Satatic: false,
	},
}

// Legacy function - replaced by GetRouter()
// Keeping for reference but should be removed after testing
/*
func GetRoute_() *mux.Router {
	// This function has been replaced by GetRouter() with automatic module and static route registration
	// All functionality moved to the new system
	return GetRouter()
}
*/

// registerRoutes registers application routes recursively
func registerRoutes(router *mux.Router, routes []Route) {
	for _, route := range routes {
		// Static routes are now handled by RegisterStaticRoutes()
		if route.Satatic {
			continue // Skip static routes - they're handled separately
		}

		// Register dynamic routes with handlers
		if route.Handler != nil {
			r := router.HandleFunc(route.Pattern, route.Handler)
			if len(route.Methods) > 0 {
				r.Methods(route.Methods...)
			}
		}

		// Register subroutes recursively
		if len(route.Subroutes) > 0 {
			sub := router.PathPrefix(route.Pattern).Subrouter()
			registerRoutes(sub, route.Subroutes)
		}
	}
}

func RegisterCustomRoutes(router *mux.Router) {
	// Serve the SPA shell for any client-side "/pages/*" deep link so that a
	// direct navigation / refresh boots the React app (which then routes
	// client-side).
	router.PathPrefix("/pages/").HandlerFunc(controllers.Index)

	// Framework endpoints that belong to no single module/page/feature:
	//   - the module list for the SPA (rights-aware; lives in controllers)
	//   - the fieldset schema API used by FieldsetProvider
	//   - the OAuth redirect/callback flow
	// Everything else (profile, login, images, papers, module CRUD) now
	// self-registers via module.AddRouteRegistrar / the module system.
	router.HandleFunc("/api/modules", controllers.ModulesAPI).Methods("GET")
	router.HandleFunc("/api/modules/{moduleId}/fieldset", module.GlobalFieldsetHandler.GetFieldset).Methods("GET")
	router.HandleFunc("/users/Auth/{authType}", auth.Auth).Methods("GET", "POST")
}

// GetRouter creates the main router with automatic module route registration
func GetRouter() *mux.Router {
	router := mux.NewRouter().StrictSlash(true)

	// Set this router as the global router for automatic module registration
	module.SetGlobalRouter(router)

	// Register static file routes (css, js, img) - no middleware applied to these
	RegisterStaticRoutes(router)

	// Register predefined application routes
	registerRoutes(router, routes)

	RegisterCustomRoutes(router)

	// Flush routes registered by pages/features via AddRouteRegistrar (profile,
	// login, images, papers, ...). They own their routes; route.go just triggers
	// their init() via imports and flushes here.
	module.FlushRouteRegistrars(router)

	// Register any remaining module routes (in case modules were loaded before SetGlobalRouter)
	module.RegisterModuleRoutes(router)

	// SPA fallback: any GET that didn't match an API / module / static route
	// serves the React shell, so client-side routes (e.g. /pages/opencv) boot on
	// direct navigation or refresh instead of 404ing. Registered last so specific
	// routes win; /api/* still 404s rather than returning HTML.
	router.PathPrefix("/").Methods("GET").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		controllers.Index(w, r)
	})

	// Attach middleware only to non-static routes
	amw := middleware.AuthenticationMiddleware{}
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if the request is for a static route - skip middleware for these
			path := r.URL.Path

			// Check against static configurations
			for _, config := range DefaultStaticConfigs {
				if len(config.URLPath) > 1 && len(path) >= len(config.URLPath) &&
					path[:len(config.URLPath)] == config.URLPath {
					// Static route: skip middleware
					next.ServeHTTP(w, r)
					return
				}
			}

			// Check legacy static routes in case any remain
			for _, route := range routes {
				if route.Satatic && len(route.Pattern) > 1 && len(path) >= len(route.Pattern) &&
					path[:len(route.Pattern)] == route.Pattern {
					// Static route: skip middleware
					next.ServeHTTP(w, r)
					return
				}
			}

			// Non-static: apply middleware
			amw.Middleware(next).ServeHTTP(w, r)
		})
	})

	return router
}
