package route

import (
	"flag"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/ws117z5/tls-rest/go/controllers"
	"github.com/ws117z5/tls-rest/go/controllers/log"
	"github.com/ws117z5/tls-rest/go/controllers/papers"
	"github.com/ws117z5/tls-rest/go/controllers/posts"
	"github.com/ws117z5/tls-rest/go/controllers/users"

	"github.com/ws117z5/tls-rest/go/lib/auth"
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
	{
		Name:    "Users",
		Pattern: "/users",
		Methods: []string{"GET"},
		Handler: users.List,
		Satatic: false,
		Subroutes: []Route{
			{
				Name:    "UsersAuth",
				Pattern: "/Auth/{authType}",
				Methods: []string{"GET", "POST"},
				Handler: auth.Auth,
				Params:  []string{"authType"},
			},
			// {
			// 	Name:    "UsersSession",
			// 	Pattern: "/session",
			// 	Methods: []string{"GET"},
			// 	Handler: auth.ManageSession,
			// },
		},
	},
	{
		Name:    "Posts",
		Pattern: "/posts",
		Methods: []string{"GET"},
		Handler: posts.List,
		Satatic: false,
		Subroutes: []Route{
			{
				Name:    "PostsCreate",
				Pattern: "/",
				Methods: []string{"POST"},
				Handler: posts.Create,
			},
			{
				Name:    "PostsView",
				Pattern: "/{postId}",
				Methods: []string{"GET"},
				Handler: posts.View,
				Params:  []string{"postId"},
			},
			{
				Name:    "PostsEdit",
				Pattern: "/{postId}",
				Methods: []string{"POST"},
				Handler: posts.Edit,
				Params:  []string{"postId"},
			},
			{
				Name:    "PostsDelete",
				Pattern: "/{postId}",
				Methods: []string{"DELETE"},
				Handler: posts.Delete,
				Params:  []string{"postId"},
			},
		},
	},
	{
		Name:    "StaticImg",
		Methods: DefaultMethod,
		Pattern: "/img/",
		Handler: nil,
		Satatic: true,
	},
	{
		Name:    "StaticJs",
		Methods: DefaultMethod,
		Pattern: "/js/",
		Handler: nil,
		Satatic: true,
	},
	{
		Name:    "StaticCss",
		Methods: DefaultMethod,
		Pattern: "/css/",
		Handler: nil,
		Satatic: true,
	},
	{
		Name:    "Metrics",
		Methods: DefaultMethod,
		Pattern: "/metrics",
		Handler: promhttp.Handler().ServeHTTP,
		Satatic: false,
	},
}

// GetRouter creates routes
// return mux/router info
func GetRoute_() *mux.Router {

	router := mux.NewRouter().StrictSlash(true)

	var dir string
	flag.StringVar(&dir, "dir", "js", "the directory to serve files from. Defaults to the current dir")
	flag.Parse()

	//http.Handle("/js/", http.StripPrefix("/js/", http.FileServer(http.Dir("./js"))))
	router.HandleFunc("/", controllers.Index)

	router.PathPrefix("/pages").HandlerFunc(controllers.Index)

	router.PathPrefix("/log").HandlerFunc(log.Log1).Methods("POST")
	router.HandleFunc("/login", controllers.Index)   //todo
	router.HandleFunc("/profile", controllers.Index) //todo

	router.HandleFunc("/papers", papers.List).Methods("GET")
	router.HandleFunc("/papers/create", papers.CreateRoom).Methods("POST")
	router.HandleFunc("/papers/{roomId}", papers.AddRoomUser).Methods("POST")
	router.HandleFunc("/papers/{roomId}", papers.ViewRoomUsers).Methods("GET")
	router.HandleFunc("/papers/{roomId}/{userId}", papers.RegisterUser).Methods("POST")
	// router.HandleFunc("/graph", controllers.Index)
	// router.HandleFunc("/imageproc", controllers.Index)
	// router.HandleFunc("/papers", controllers.Index)

	//router.HandleFunc("/opencv", opencv.Init).Methods("GET", "POST")

	//router.HandleFunc("/users/GetInfo/{authType}", users.GetInfo).Methods("GET")
	router.HandleFunc("/users", users.List).Methods("GET")
	//router.HandleFunc("/users", controllers.Index).Methods("GET").
	router.HandleFunc("/users/Auth/{authType}", auth.Auth).Methods("GET", "POST")
	//router.HandleFunc("/users/session", auth.ManageSession).Methods("GET")
	//router.HandleFunc("/users/AuthResponse/{authType}", auth.AuthResponse).Methods("GET")

	router.HandleFunc("/posts", posts.List).Methods("GET")
	router.HandleFunc("/posts/", posts.Create).Methods("POST")
	router.HandleFunc("/posts/{postId}", posts.View).Methods("GET")
	router.HandleFunc("/posts/{postId}", posts.Edit).Methods("POST")
	router.HandleFunc("/posts/{postId}", posts.Delete).Methods("DELETE")

	router.PathPrefix("/img/").Handler(http.FileServer(http.Dir(".")))
	router.PathPrefix("/js/").Handler(http.FileServer(http.Dir(".")))
	//router.PathPrefix("/css/").Handler(http.FileServer(http.Dir(".")))
	//router.PathPrefix("/css/").Handler(http.StripPrefix("/css/", http.FileServer(http.FS(assets))))
	router.PathPrefix("/css/").Handler(http.StripPrefix("/css/", http.FileServer(http.Dir("css"))))

	router.Handle("/metrics", promhttp.Handler())
	//router.PathPrefix("/users/").Handler(http.FileServer(http.Dir(".")))

	amw := middleware.AuthenticationMiddleware{}
	//amw.Populate()
	router.Use(amw.Middleware)
	//TODO Add midleware authenication bu cookie tokens

	//TODO Convert {type} to enum const

	var usersRouter = router.PathPrefix("/users").Subrouter()
	usersRouter.HandleFunc("/Auth/{type}/", controllers.Index).Methods("GET")
	//user]sRouter.HandleFunc("/Auth/{type}/", users.Auth).Methods("GET")

	return router
}

// Helper to register routes recursively
func registerRoutes(router *mux.Router, routes []Route) {
	for _, route := range routes {
		if route.Satatic {
			// Static routes: no middleware, just file server
			router.PathPrefix(route.Pattern).Handler(http.StripPrefix(route.Pattern, http.FileServer(http.Dir(route.Pattern[1:]))))
			continue
		}
		if route.Handler != nil {
			r := router.HandleFunc(route.Pattern, route.Handler)
			if len(route.Methods) > 0 {
				r.Methods(route.Methods...)
			}
		}
		if len(route.Subroutes) > 0 {
			sub := router.PathPrefix(route.Pattern).Subrouter()
			registerRoutes(sub, route.Subroutes)
		}
	}
}

// In GetRouter:
func GetRouter() *mux.Router {
	router := mux.NewRouter().StrictSlash(true)
	registerRoutes(router, routes)

	// Attach middleware only to non-static routes
	amw := middleware.AuthenticationMiddleware{}
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if the request is for a static route
			path := r.URL.Path
			for _, route := range routes {
				if route.Satatic && len(route.Pattern) > 1 && len(path) >= len(route.Pattern) && path[:len(route.Pattern)] == route.Pattern {
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
