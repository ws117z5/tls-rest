//go:generate go run init/init.go
package main

import (

	//"log"

	"fmt"
	"os"
	"time"

	//pgdb "tls-rest/go/lib/db/pgdb"

	constants "tls-rest/go/constants"
	input "tls-rest/go/lib/subroutine/input"
	server "tls-rest/go/lib/subroutine/server"
	"tls-rest/leet"

	"tls-rest/go/lib/log"

	"github.com/prometheus/client_golang/prometheus"
)

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
	// Metrics have to be registered to be exposed:
	prometheus.MustRegister(cpuTemp)
	prometheus.MustRegister(hdFailures)
	//os.Setenv("GODEBUG", os.Getenv("GODEBUG")+",tls13=1")
}

func main() {

	leet.Run()

	startTime := time.Now()

	// Fail fast if any absolutely-required configuration is missing.
	if err := constants.ValidateRequired(); err != nil {
		fmt.Fprintln(os.Stderr, "configuration error:", err)
		os.Exit(1)
	}

	// Log application startup
	log.LogSystemEvent("Application starting up", log.LogLevelInfo, map[string]interface{}{
		"start_time": startTime,
		"pid":        os.Getpid(),
	})

	cpuTemp.Set(65.3)
	hdFailures.With(prometheus.Labels{"device": "/dev/disk1"}).Inc()

	// Log metrics initialization
	log.LogSystemEvent("Prometheus metrics initialized", log.LogLevelInfo, map[string]interface{}{
		"cpu_temp":         65.3,
		"metrics_endpoint": "/metrics",
	})

	// The Handler function provides a default handler to expose metrics
	// via an HTTP server. "/metrics" is the usual endpoint for that.

	//inside router
	//http.Handle("/metrics", promhttp.Handler())
	//log.Fatal(http.ListenAndServe(":8080", nil))

	log.LogSystemEvent("Starting input command reader", log.LogLevelInfo, nil)
	go input.ReadCommand()

	log.LogSystemEvent("Starting HTTP server", log.LogLevelInfo, map[string]interface{}{
		"startup_duration_ms": time.Since(startTime).Seconds() * 1000,
	})

	// RunServer blocks until an interrupt/termination signal arrives, then
	// drains in-flight requests and returns (graceful shutdown).
	server.RunServer()

	log.LogSystemEvent("Application shut down", log.LogLevelInfo, map[string]interface{}{
		"uptime_seconds": time.Since(startTime).Seconds(),
	})
}
