//go:generate go run init/init.go
package main

import (

	//"log"

	"os"
	"os/signal"
	"syscall"

	//pgdb "github.com/ws117z5/tls-rest/go/lib/db/pgdb"

	input "github.com/ws117z5/tls-rest/go/lib/subroutine/input"
	server "github.com/ws117z5/tls-rest/go/lib/subroutine/server"

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

type Test struct {
	Name string
}

func (t *Test) String() string {
	return t.Name
}

type TestChild struct {
	Test
}

func main() {

	cpuTemp.Set(65.3)
	hdFailures.With(prometheus.Labels{"device": "/dev/disk1"}).Inc()

	// The Handler function provides a default handler to expose metrics
	// via an HTTP server. "/metrics" is the usual endpoint for that.

	//inside router
	//http.Handle("/metrics", promhttp.Handler())
	//log.Fatal(http.ListenAndServe(":8080", nil))

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		//TODO Cleanup
		//cleanupAllTheThings()
		os.Exit(0)
	}()

	go input.ReadCommand()
	server.RunServer()
}
