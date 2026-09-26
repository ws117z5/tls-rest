// Package app orchestrates process startup: modules/pages self-register into
// it via RegisterModule/RegisterPage (see go/bootstrap), and Run drives config, logging, routing, and the server.
package app

import (
	"fmt"
	"os"
	"strings"
	"time"

	constants "tls-rest/go/app/constants"
	"tls-rest/go/engine/controllers/accesslog"
	"tls-rest/go/engine/controllers/db/pgdb"
	"tls-rest/go/engine/controllers/httpx"
	"tls-rest/go/engine/controllers/log"
	"tls-rest/go/engine/controllers/module"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
)

// App holds every registered module and page, in registration order.
type App struct {
	modules []module.ModuleInterface
	pages   []*module.PageAbstract
}

// Default is the process's single App instance, target of the package-level RegisterModule/RegisterPage self-registration calls.
var Default = &App{}

// RegisterModule initializes m and adds it to the app, in call order. Call
// once from a module package's own init() — never call m.Initialize itself.
func RegisterModule(m module.Initializer, tableName string) {
	m.Initialize(tableName)
	Default.modules = append(Default.modules, m)
}

// RegisterPage is RegisterModule's counterpart for pages: calls p.Initialize()
// and adds p to the app, in call order. Call once from a page package's own
// init().
func RegisterPage(p *module.PageAbstract) {
	p.Initialize()
	Default.pages = append(Default.pages, p)
}

// Modules returns every registered module, in registration order (a
// defensive copy).
func Modules() []module.ModuleInterface {
	out := make([]module.ModuleInterface, len(Default.modules))
	copy(out, Default.modules)
	return out
}

// Pages returns every registered page, in registration order (a defensive
// copy).
func Pages() []*module.PageAbstract {
	out := make([]*module.PageAbstract, len(Default.pages))
	copy(out, Default.pages)
	return out
}

// RegisterRoutes wires every registered page's and module's routes onto
// router in two passes across all modules — every Absolute custom route
// first, then every module's own /{id} subrouter — so no module's literal
// route can lose a match to another module's /{id} wildcard regardless of load order.
func RegisterRoutes(router *mux.Router) {
	module.FlushRouteRegistrars(router)

	for _, m := range Default.modules {
		registerModuleAbsoluteRoutes(router, m)
	}
	for _, m := range Default.modules {
		registerModuleSubrouter(router, m)
	}
}

func registerModuleAbsoluteRoutes(router *mux.Router, m module.ModuleInterface) {
	cr, ok := m.(module.CustomRouter)
	if !ok {
		return
	}
	for _, rt := range cr.GetCustomRoutes() {
		if !rt.Absolute {
			continue
		}
		methods := rt.Methods
		if len(methods) == 0 {
			methods = []string{"GET"}
		}
		router.HandleFunc(rt.Path, rt.Handler).Methods(methods...)
	}
}

func registerModuleSubrouter(router *mux.Router, m module.ModuleInterface) {
	moduleID := m.GetID()
	sub := router.PathPrefix("/" + moduleID).Subrouter()
	sub.HandleFunc("", m.List).Methods("GET")
	sub.HandleFunc("", m.Create).Methods("POST")

	if cr, ok := m.(module.CustomRouter); ok {
		for _, rt := range cr.GetCustomRoutes() {
			if rt.Absolute {
				continue
			}
			methods := rt.Methods
			if len(methods) == 0 {
				methods = []string{"GET"}
			}
			sub.HandleFunc(rt.Path, rt.Handler).Methods(methods...)
		}
	}

	sub.HandleFunc("/{id}", m.View).Methods("GET")
	sub.HandleFunc("/{id}", m.Edit).Methods("PUT", "PATCH")
	sub.HandleFunc("/{id}", m.Delete).Methods("DELETE")
}

var (
	cpuTemp = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "cpu_temperature_celsius",
		Help: "Current temperature of the CPU.",
	})
	hdFailures = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hd_errors_total",
			Help: "Number of hard-disk errors.",
		},
		[]string{"device"},
	)
)

func init() {
	prometheus.MustRegister(cpuTemp)
	prometheus.MustRegister(hdFailures)
}

// Run is the process entry point: validates config, inits logging/metrics,
// then blocks in startServer. Params, not imports, dodge a cycle back through
// auth -> a module -> app.RegisterModule; wires module.OnRightsChange/OnConfigChange.
func Run(startServer func(registerRoutes func(*mux.Router)), startCLI func(), onRightsChange func(), onConfigChange func()) {
	module.OnRightsChange = onRightsChange
	module.OnConfigChange = onConfigChange

	startTime := time.Now()

	if err := constants.ValidateRequired(); err != nil {
		fmt.Fprintln(os.Stderr, "configuration error:", err)
		os.Exit(1)
	}

	httpx.SetTrustedHosts(strings.Split(constants.Env("APP_HOSTS", "localhost"), ","))
	httpx.SetTrustedProxies(strings.Split(constants.Env("TRUSTED_PROXIES", ""), ","))

	accesslog.Init()

	var logDB log.DB
	if database, dberr := pgdb.GetInstance(); dberr == nil {
		logDB = database
	} else {
		fmt.Fprintln(os.Stderr, "logging: database unavailable:", dberr)
	}
	log.Init(logDB)

	registered := make(map[string]bool, len(Default.modules))
	for _, m := range Default.modules {
		registered[m.GetID()] = true
	}
	for _, cerr := range constants.Config.Validate(registered) {
		log.LogSystemEvent("config validation: "+cerr.Error(), log.LogLevelError, nil)
	}

	log.LogSystemEvent("Application starting up", log.LogLevelInfo, map[string]interface{}{
		"start_time": startTime,
		"pid":        os.Getpid(),
	})

	cpuTemp.Set(65.3)
	hdFailures.With(prometheus.Labels{"device": "/dev/disk1"}).Inc()
	log.LogSystemEvent("Prometheus metrics initialized", log.LogLevelInfo, map[string]interface{}{
		"cpu_temp":         65.3,
		"metrics_endpoint": "/metrics",
	})

	if startCLI != nil {
		log.LogSystemEvent("Starting input command reader", log.LogLevelInfo, nil)
		go startCLI()
	}

	log.LogSystemEvent("Starting HTTP server", log.LogLevelInfo, map[string]interface{}{
		"startup_duration_ms": time.Since(startTime).Seconds() * 1000,
	})

	startServer(RegisterRoutes)

	log.LogSystemEvent("Application shut down", log.LogLevelInfo, map[string]interface{}{
		"uptime_seconds": time.Since(startTime).Seconds(),
	})
}
