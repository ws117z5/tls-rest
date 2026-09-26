package httpx

import (
	"net/http"
	"strconv"
	"sync"
	"time"
)

type hitWindow struct {
	count int
	reset time.Time
}

var (
	rateMu     sync.Mutex
	rateHits   = map[string]*hitWindow{}
	ratePruned time.Time
)

// Allow counts one hit on key and reports whether it is within max hits per window (fixed window, in memory).
func Allow(key string, max int, per time.Duration) bool {
	now := time.Now()
	rateMu.Lock()
	defer rateMu.Unlock()

	if now.Sub(ratePruned) > time.Minute {
		for k, w := range rateHits {
			if now.After(w.reset) {
				delete(rateHits, k)
			}
		}
		ratePruned = now
	}

	w := rateHits[key]
	if w == nil || now.After(w.reset) {
		w = &hitWindow{reset: now.Add(per)}
		rateHits[key] = w
	}
	w.count++
	return w.count <= max
}

// RetryAfter answers 429 with a Retry-After hint.
func RetryAfter(w http.ResponseWriter, per time.Duration) {
	w.Header().Set("Retry-After", strconv.Itoa(int(per.Seconds())))
	http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
}

// RateLimit wraps h so each client IP gets at most max requests per window on this named route.
func RateLimit(name string, max int, per time.Duration, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !Allow(name+"|"+ClientIP(r), max, per) {
			RetryAfter(w, per)
			return
		}
		h(w, r)
	}
}
