package module

import (
	"sort"
	"strings"
	"sync"
)

// Endpoint registry: the authoritative, self-populated set of URL paths that map
// to DATA endpoints (module CRUD roots, page endpoints, feature routes) rather
// than SPA pages. Modules and pages register themselves from Initialize();
// features that own arbitrary route trees register explicitly. The auth
// middleware consults this instead of a hardcoded path list, so adding a module
// or page never requires touching the middleware.

var (
	endpointMu       sync.RWMutex
	endpointPrefixes = map[string]struct{}{}
)

// RegisterEndpointPrefix records path (normalized to its leading static
// segment) as a data endpoint. Idempotent and safe for concurrent use.
func RegisterEndpointPrefix(path string) {
	p := staticPrefix(path)
	if p == "/" {
		return
	}
	endpointMu.Lock()
	endpointPrefixes[p] = struct{}{}
	endpointMu.Unlock()
}

// IsRegisteredEndpoint reports whether path targets a registered data endpoint,
// matching on whole segments: "/posts" matches "/posts" and "/posts/5" but not
// "/postscript".
func IsRegisteredEndpoint(path string) bool {
	endpointMu.RLock()
	defer endpointMu.RUnlock()
	for p := range endpointPrefixes {
		if path == p || strings.HasPrefix(path, p+"/") {
			return true
		}
	}
	return false
}

// RegisteredEndpointPrefixes returns the registered prefixes, sorted (for
// startup logging / diagnostics).
func RegisteredEndpointPrefixes() []string {
	endpointMu.RLock()
	defer endpointMu.RUnlock()
	out := make([]string, 0, len(endpointPrefixes))
	for p := range endpointPrefixes {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// staticPrefix normalizes a route path to its leading static portion: ensures a
// single leading slash, drops a trailing slash, and truncates at the first mux
// path variable. e.g. "/users/Auth/{authType}" -> "/users/Auth",
// "posts" -> "/posts", "/api/netmap/events" -> "/api/netmap/events".
func staticPrefix(path string) string {
	if i := strings.IndexByte(path, '{'); i >= 0 {
		path = path[:i]
	}
	return "/" + strings.Trim(path, "/")
}
