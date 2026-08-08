package route

import (
	"net/http"

	"github.com/gorilla/mux"

	"tls-rest/go/controllers"
	"tls-rest/go/controllers/log"
	"tls-rest/go/controllers/papers"
	"tls-rest/go/controllers/postimages"

	// Import modules to trigger their init() functions for automatic registration
	_ "tls-rest/go/controllers/modulerights"
	_ "tls-rest/go/controllers/posts"
	_ "tls-rest/go/controllers/usergroups"
	_ "tls-rest/go/controllers/users"

	"tls-rest/go/lib/module"
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
				Name:    "PapersReport",
				Pattern: "/{roomId}/report",
				Methods: []string{"POST"},
				Handler: papers.ReportLink,
				Params:  []string{"roomId"},
			},
			{
				Name:    "PapersPlan",
				Pattern: "/{roomId}/plan",
				Methods: []string{"GET"},
				Handler: papers.GetPlan,
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

func RegisterCustomRoutes(router *mux.Router) {
	// Serve the SPA shell for any client-side "/pages/*" deep link so that a
	// direct navigation / refresh boots the React app (which then routes
	// client-side). The previous per-page patterns were missing the leading
	// "/" and therefore never matched, 404-ing on refresh.
	router.PathPrefix("/pages/").HandlerFunc(controllers.Index)

	// Fieldset API used by the React FieldsetProvider. The handler already
	// existed (module.GlobalFieldsetHandler) but was never wired to the router,
	// so GET /api/modules/{moduleId}/fieldset returned 404.
	router.HandleFunc("/api/modules", module.GlobalFieldsetHandler.GetModules).Methods("GET")
	router.HandleFunc("/api/modules/{moduleId}/fieldset", module.GlobalFieldsetHandler.GetFieldset).Methods("GET")

	// Post images: DB-backed upload/list/serve. Namespaced under /api/ to avoid
	// colliding with the generic module route /posts/{id}.
	router.HandleFunc("/api/posts/images", postimages.Upload).Methods("POST")
	router.HandleFunc("/api/posts/images", postimages.List).Methods("GET")
	router.HandleFunc("/api/posts/images/{id}", postimages.Serve).Methods("GET")
	//router.HandleFunc("/opencv", opencv.Init).Methods("GET", "POST", "OPTIONS")

	//router.HandleFunc("/users/GetInfo/{authType}", users.GetInfo).Methods("GET")
	//router.HandleFunc("/users", controllers.Index).Methods("GET").
	//router.HandleFunc("/users/Auth/{authType}", auth.Auth).Methods("GET", "POST")
	//router.HandleFunc("/users/session", auth.ManageSession).Methods("GET")
	//router.HandleFunc("/users/AuthResponse/{authType}", auth.AuthResponse).Methods("GET")
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
