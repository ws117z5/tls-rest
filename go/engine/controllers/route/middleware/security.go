package middleware

import (
	"net/http"

	"tls-rest/go/engine/controllers/httpx"
)

// CSRFGuard rejects state-changing requests that ride on the session cookie
// but don't come from this app: the Origin (when sent) must be this host or an
// allowlisted one, and the request must carry X-Request-Type: api, a header a
// cross-site page can't add without a CORS preflight. Requests without the
// session cookie (bearer-token clients) carry no ambient authority and pass.
func CSRFGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		if _, err := r.Cookie("X-Session-ID"); err != nil {
			next.ServeHTTP(w, r)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" && !httpx.SameOrigin(origin, r) && !httpx.TrustedOrigin(origin) {
			http.Error(w, "cross-site request blocked", http.StatusForbidden)
			return
		}
		if r.Header.Get("X-Request-Type") != "api" {
			http.Error(w, "missing X-Request-Type header", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// SecureHeaders sets a baseline set of security response headers (including a
// per-response Content-Security-Policy nonce) on every response. It is applied
// as the outermost handler so it also covers static assets.
func SecureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()

		// Force HTTPS for a year. Browsers ignore HSTS for localhost/IP hosts,
		// so this is safe during local development.
		h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")

		// Stop browsers from MIME-sniffing responses away from the declared type.
		h.Set("X-Content-Type-Options", "nosniff")

		// Minimise referrer leakage to third parties.
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Clickjacking protection (legacy header + its modern CSP equivalent).
		h.Set("X-Frame-Options", "DENY")

		// No inline script runs without this response's nonce, so injected markup and javascript: URLs are inert.
		// 'unsafe-eval' stays because the Graph/ArrayIterator pages run new Function and graphviz uses wasm.
		nonce := httpx.NewNonce()
		h.Set("Content-Security-Policy", "default-src 'self'; "+
			"script-src 'self' 'nonce-"+nonce+"' 'unsafe-eval' 'wasm-unsafe-eval' https://accounts.google.com; "+
			"style-src 'self' 'unsafe-inline' https://accounts.google.com; "+
			"img-src 'self' data: blob: https:; "+
			"font-src 'self' data:; "+
			"connect-src 'self' https: http: ws: wss:; "+
			"media-src 'self' blob:; "+
			"worker-src 'self' blob:; "+
			"frame-src https://accounts.google.com; "+
			"object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'")

		next.ServeHTTP(w, httpx.WithNonce(r, nonce))
	})
}
