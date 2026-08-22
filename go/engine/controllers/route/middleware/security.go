package middleware

import "net/http"

// SecureHeaders sets a baseline set of security response headers on every
// response. It is applied as the outermost handler so it also covers static
// assets. Tune these for your deployment (in particular Content-Security-Policy,
// which is intentionally left commented out because a strict policy can break
// inline scripts such as Google Sign-In).
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

		// A conservative Content-Security-Policy. Enable and tailor once the
		// script/style sources for the SPA and any third-party widgets are known.
		// h.Set("Content-Security-Policy", "default-src 'self'; frame-ancestors 'none'")

		next.ServeHTTP(w, r)
	})
}
