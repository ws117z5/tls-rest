package statistics

import (
	"context"
	"net/http"
	"strings"
	"time"

	"tls-rest/go/engine/controllers/db/cache"
	"tls-rest/go/engine/controllers/db/pgdb"
	"tls-rest/go/engine/controllers/functions"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// RefreshPresets are the allowed client polling intervals (seconds) for the
// live metrics panel — fixed here, not left to arbitrary client input.
var RefreshPresets = []int{5, 15, 30, 60}

// metricValue sums every series of a metric family — the family's total
// across all label combinations, whether it's a counter or a gauge.
func metricValue(f *dto.MetricFamily) float64 {
	if f == nil {
		return 0
	}
	var total float64
	for _, m := range f.GetMetric() {
		if c := m.GetCounter(); c != nil {
			total += c.GetValue()
		} else if g := m.GetGauge(); g != nil {
			total += g.GetValue()
		}
	}
	return total
}

// labelValue returns one series' value for a family carrying labelName=want
// (e.g. http_requests_by_module_total{module="posts"}), or 0 if absent.
func labelValue(f *dto.MetricFamily, labelName, want string) float64 {
	if f == nil {
		return 0
	}
	for _, m := range f.GetMetric() {
		for _, lp := range m.GetLabel() {
			if lp.GetName() == labelName && lp.GetValue() == want {
				if c := m.GetCounter(); c != nil {
					return c.GetValue()
				}
				if g := m.GetGauge(); g != nil {
					return g.GetValue()
				}
			}
		}
	}
	return 0
}

// Live handles GET /api/statistics/live?module=. ADMIN ONLY. Returns one
// current snapshot (Prometheus counters + a DB size query) — cumulative
// values, so the client derives a rate itself by diffing consecutive polls.
func Live(w http.ResponseWriter, r *http.Request) {
	s := cache.SessionFromContext(r.Context())
	if s == nil || !s.IsAdmin {
		functions.JSONError(w, http.StatusForbidden, "admin only")
		return
	}
	moduleFilter := strings.TrimSpace(r.URL.Query().Get("module"))

	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "metrics unavailable")
		return
	}
	byName := map[string]*dto.MetricFamily{}
	for _, f := range families {
		byName[f.GetName()] = f
	}

	requests := metricValue(byName["http_requests_total"])
	if moduleFilter != "" {
		requests = labelValue(byName["http_requests_by_module_total"], "module", moduleFilter)
	}

	heapBytes := metricValue(byName["go_memstats_heap_inuse_bytes"])
	sysBytes := metricValue(byName["go_memstats_sys_bytes"])
	engineOverhead := sysBytes - heapBytes
	if engineOverhead < 0 {
		engineOverhead = 0
	}

	functions.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"time":                  time.Now().UTC().Format(time.RFC3339),
		"requests_total":        requests,
		"db_queries_total":      metricValue(byName["db_queries_total"]),
		"cpu_seconds_total":     metricValue(byName["process_cpu_seconds_total"]),
		"memory_bytes":          metricValue(byName["process_resident_memory_bytes"]),
		"app_heap_bytes":        heapBytes,
		"engine_overhead_bytes": engineOverhead,
		"db_size_bytes":         dbSizeBytes(r.Context()),
		"refresh_presets":       RefreshPresets,
	})
}

// dbSizeBytes returns the current database's on-disk size (not memory — the
// closest thing to a "database" resource figure reachable over a normal
// connection, without superuser/pg_monitor access to server-side memory).
func dbSizeBytes(ctx context.Context) float64 {
	db, err := pgdb.GetInstanceCtx(ctx)
	if err != nil {
		return 0
	}
	row, err := db.GetOne("SELECT pg_database_size(current_database()) AS n")
	if err != nil || row == nil {
		return 0
	}
	return functions.Coerce[float64](row["n"])
}
