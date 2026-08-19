// Package accesslog captures one record per HTTP request into Postgres
// (asynchronously, off the request path), exports Prometheus HTTP metrics, and
// provides the IP/CIDR access-control decision used by the middleware. It is the
// shared backend for the access-logs module, the metrics page, the analytics
// dashboard, and the access-control page.
package accesslog

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	pgdb "tls-rest/go/lib/db/pgdb"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Entry is one access-log record. Status and Duration are filled in after the
// handler runs (see the middleware's status-capturing writer).
type Entry struct {
	Time         time.Time
	Method       string
	Path         string
	Status       int
	DurationMS   float64
	UserID       int
	SessionID    string
	IP           string
	UserAgent    string
	Module       string
	Action       string
	Blocked      bool
	DeniedReason string
}

// Prometheus HTTP metrics, registered on the default registry that /metrics
// already serves. Path is intentionally NOT a label (unbounded cardinality);
// per-path analytics come from the access_log table instead.
var (
	requestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total HTTP requests processed, by method and status class.",
	}, []string{"method", "status"})

	requestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request latency in seconds, by method.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method"})

	blockedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_blocked_total",
		Help: "Requests denied before handling, by reason.",
	}, []string{"reason"})
)

var (
	queue     chan Entry
	startOnce sync.Once
)

// Init starts the background writer and primes the rule cache. Call once at
// startup after config is validated (the DB itself connects lazily).
func Init() {
	startOnce.Do(func() {
		queue = make(chan Entry, 4096)
		go worker()
		ReloadRules()
	})
}

// Record queues an entry for persistence and updates the live metrics. Safe for
// concurrent use and never blocks the request: if the buffer is full the DB row
// is dropped (metrics are still counted) rather than stalling the handler.
func Record(e Entry) {
	statusClass := "0"
	if e.Status > 0 {
		statusClass = string(rune('0'+e.Status/100)) + "xx"
	}
	requestsTotal.WithLabelValues(e.Method, statusClass).Inc()
	requestDuration.WithLabelValues(e.Method).Observe(e.DurationMS / 1000.0)
	if e.Blocked {
		reason := e.DeniedReason
		if reason == "" {
			reason = "unknown"
		}
		blockedTotal.WithLabelValues(reason).Inc()
	}

	if queue == nil {
		return // Init not called; metrics still recorded above
	}
	select {
	case queue <- e:
	default:
		// buffer full — drop the DB write to protect latency
	}
}

func worker() {
	for e := range queue {
		persist(e)
	}
}

func persist(e Entry) {
	db, err := pgdb.GetInstance()
	if err != nil {
		return
	}
	row := map[string]interface{}{
		"ts":          e.Time,
		"method":      e.Method,
		"path":        e.Path,
		"status":      e.Status,
		"duration_ms": e.DurationMS,
		"session_id":  nullIfEmpty(e.SessionID),
		"ip":          nullIfEmpty(e.IP),
		"user_agent":  nullIfEmpty(e.UserAgent),
		"module":      nullIfEmpty(e.Module),
		"action":      nullIfEmpty(e.Action),
		"blocked":     e.Blocked,
	}
	if e.UserID > 0 {
		row["user_id"] = e.UserID
	}
	if e.DeniedReason != "" {
		row["denied_reason"] = e.DeniedReason
	}
	_, _ = db.InsertRow("access_log", row)
}

// ClientIP extracts the best-effort client IP, honouring a terminating proxy
// (X-Forwarded-For first hop, then X-Real-IP) and falling back to RemoteAddr.
func ClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	if xr := r.Header.Get("X-Real-IP"); xr != "" {
		return strings.TrimSpace(xr)
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
