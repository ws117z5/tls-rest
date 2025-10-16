package route

import (
	"net/http"

	"github.com/gorilla/mux"

	"github.com/ws117z5/tls-rest/go/controllers"
	"github.com/ws117z5/tls-rest/go/controllers/log"
	"github.com/ws117z5/tls-rest/go/controllers/papers"

	// Import modules to trigger their init() functions for automatic registration
	_ "github.com/ws117z5/tls-rest/go/controllers/modulerights"
	_ "github.com/ws117z5/tls-rest/go/controllers/posts"
	_ "github.com/ws117z5/tls-rest/go/controllers/usergroups"
	_ "github.com/ws117z5/tls-rest/go/controllers/users"

	"github.com/ws117z5/tls-rest/go/lib/module"
	middleware "github.com/ws117z5/tls-rest/go/lib/route/middlware"

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
	{
		Name:    "Papers",
		Pattern: "/papers",
		Methods: []string{"GET"},
		Handler: papers.List,
		Satatic: false,
		Subroutes: []Route{
			{
				Name:    "PapersCreate",
				Pattern: "/create",
				Methods: []string{"POST"},
				Handler: papers.CreateRoom,
			},
			{
				Name:    "PapersAddUser",
				Pattern: "/{roomId}",
				Methods: []string{"POST"},
				Handler: papers.AddRoomUser,
				Params:  []string{"roomId"},
			},
			{
				Name:    "PapersViewUsers",
				Pattern: "/{roomId}",
				Methods: []string{"GET"},
				Handler: papers.ViewRoomUsers,
				Params:  []string{"roomId"},
			},
			{
				Name:    "PapersRegisterUser",
				Pattern: "/{roomId}/{userId}",
				Methods: []string{"POST"},
				Handler: papers.RegisterUser,
				Params:  []string{"roomId", "userId"},
			},
		},
	},
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

// GetRouter creates the main router with automatic module route registration
func GetRouter() *mux.Router {
	router := mux.NewRouter().StrictSlash(true)

	// Set this router as the global router for automatic module registration
	module.SetGlobalRouter(router)

	// Register static file routes (css, js, img) - no middleware applied to these
	RegisterStaticRoutes(router)

	// Register predefined application routes
	registerRoutes(router, routes)

	// Register any remaining module routes (in case modules were loaded before SetGlobalRouter)
	module.RegisterModuleRoutes(router)

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
