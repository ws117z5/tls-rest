// Package querystats counts DB queries and duration per HTTP request, so an
// admin can see how expensive a click was. Totals are carried back on the
// same response (as headers, injected by the middleware once the handler
// finishes) rather than published to a side channel and re-fetched — there is
// no second request to race against, so a snapshot can never miss its window.
package querystats

import (
	"context"
	"sync/atomic"
	"time"
)

// Stats accumulates one request's query count/duration.
type Stats struct {
	count int64
	nanos int64
}

func (s *Stats) add(d time.Duration) {
	atomic.AddInt64(&s.count, 1)
	atomic.AddInt64(&s.nanos, int64(d))
}

// Count and Millis read the current totals, for the middleware to inject into
// response headers once the handler has finished.
func (s *Stats) Count() int64 { return atomic.LoadInt64(&s.count) }
func (s *Stats) Millis() float64 {
	return float64(atomic.LoadInt64(&s.nanos)) / float64(time.Millisecond)
}

type ctxKey struct{}

// NewContext attaches a fresh Stats to ctx for pgdb to record into.
func NewContext(ctx context.Context) (context.Context, *Stats) {
	s := &Stats{}
	return context.WithValue(ctx, ctxKey{}, s), s
}

// Track records one query's duration against ctx's Stats; a no-op when ctx
// carries none (background/startup code with no request behind it).
func Track(ctx context.Context, d time.Duration) {
	if ctx == nil {
		return
	}
	if s, ok := ctx.Value(ctxKey{}).(*Stats); ok {
		s.add(d)
	}
}

// excludedPaths are background requests never instrumented, so a debounced
// call that outlives the click that triggered it doesn't dilute its totals.
var excludedPaths = map[string]bool{
	"/api/i18n/resolve":         true, // debounced 150ms, so it settles after real requests on most pages
	"/api/messages/unread-count": true, // background poll (Menu.tsx), not a user action
}

// Excluded reports whether path should be skipped by the middleware.
func Excluded(path string) bool {
	return excludedPaths[path]
}

// RequestHeader is the request header the client sets (axios interceptor) to
// opt a call into instrumentation; the middleware still requires an admin
// session regardless of what a request claims.
const RequestHeader = "X-Query-Stats"

// CountHeader and MillisHeader are the response headers the middleware sets
// with this request's totals.
const (
	CountHeader  = "X-Query-Stats-Count"
	MillisHeader = "X-Query-Stats-Ms"
)
