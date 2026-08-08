package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"tls-rest/go/controllers"
	. "tls-rest/go/lib/auth"
	"tls-rest/go/lib/db/cache"
	"tls-rest/go/lib/log"
)

/*
//GetMiddleware Returns Middleware for httpRequests
func GetMiddleware() AuthenticationMiddleware {
	amw := AuthenticationMiddleware{}
	amw.Populate()

	return amw
}
*/

// AuthenticationMiddleware struct
type AuthenticationMiddleware struct {
	sessionCache *cache.Session
}

func isAPICall(r *http.Request) bool {
	requestType := r.Header.Get("X-Request-Type")
	uri := r.RequestURI

	// Explicitly marked as API by the frontend (set on every axios request).
	if requestType == "api" {
		return true
	}

	// Static assets should NOT be treated as API calls
	if strings.HasPrefix(uri, "/js/") ||
		strings.HasPrefix(uri, "/css/") ||
		strings.HasPrefix(uri, "/img/") ||
		strings.HasPrefix(uri, "/favicon") ||
		strings.HasPrefix(uri, "/metrics") {
		return false
	}

	// OAuth callbacks are browser GETs that must run the auth handler, not SSR.
	if strings.HasPrefix(uri, "/users/Auth") {
		return true
	}

	// The /api/ namespace serves both XHR JSON (fieldset) and browser-loaded
	// resources (post images), so those handlers must always run.
	if strings.HasPrefix(uri, "/api/") {
		return true
	}

	// Does the request itself look like an API call (XHR / fetch / JSON accept /
	// a mutating method) rather than a browser document navigation?
	acceptHeader := r.Header.Get("Accept")
	isLikelyAPI := strings.Contains(acceptHeader, "application/json") ||
		r.Header.Get("X-Requested-With") == "XMLHttpRequest" ||
		(r.Method != http.MethodGet && r.Method != http.MethodHead)

	// Papers + module data endpoints are API ONLY for genuine API requests, so a
	// browser navigating straight to e.g. /papers or /posts is not served raw
	// JSON (it is redirected home instead — see the middleware below).
	if isLikelyAPI && isModuleEndpoint(uri) {
		return true
	}

	// Everything else is a page/SSR request.
	return false
}

// isModuleEndpoint reports whether a path is a JSON data endpoint (papers or a
// module CRUD route) rather than an SPA page.
func isModuleEndpoint(path string) bool {
	return strings.HasPrefix(path, "/papers") ||
		strings.HasPrefix(path, "/posts") ||
		strings.HasPrefix(path, "/users") ||
		strings.HasPrefix(path, "/user_groups") ||
		strings.HasPrefix(path, "/module_rights")
}

// Middleware dlv fails here
// TODO reinstall llvm on osx, test dlv for errors, for now testing only in
func (amw *AuthenticationMiddleware) Middleware(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()

		// Check or set session cookie
		ci := ManageSession(w, r)
		ctx := context.WithValue(r.Context(), SESSION_KEY, ci)

		// Log the incoming request
		var userID *int
		if ci.UserID > 0 {
			userID = &ci.UserID
		}

		// Generate session ID from cookie or create one
		sessionID := getSessionID(r)
		requestID := log.LogRequest(r, userID, sessionID)

		// Log session event
		log.LogAuthEvent("session_check", "Session validated", userID, sessionID, true, map[string]interface{}{
			"request_id": requestID,
			"path":       r.URL.Path,
		})

		if isAPICall(r) {
			// --- Module rights check ---
			moduleName := getModuleNameFromPath(r.URL.Path)
			requiredRight := "view" // or "edit", etc. -- set as needed

			if !HasModuleRight(ci.UserID, moduleName, requiredRight) {
				if r.URL.Path == "/login" || r.URL.Path == "/" {
					// Log response
					duration := time.Since(startTime).Seconds() * 1000
					log.LogResponse(requestID, http.StatusOK, duration, userID)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
				// Log authorization failure
				log.LogAuthEvent("authorization_failed", "User lacks required module rights", userID, sessionID, false, map[string]interface{}{
					"module":         moduleName,
					"required_right": requiredRight,
					"request_id":     requestID,
				})

				duration := time.Since(startTime).Seconds() * 1000
				log.LogResponse(requestID, http.StatusUnauthorized, duration, userID)
				controllers.Error(w, r.WithContext(ctx), http.StatusUnauthorized)
				return
			}
			// --- End module rights check ---

			// Log successful authorization
			log.LogModuleEvent(moduleName, requiredRight, "Module access granted", userID, sessionID, map[string]interface{}{
				"request_id": requestID,
			})

			// Authenticated and authorized, proceed to API handler (JSON response)
			duration := time.Since(startTime).Seconds() * 1000
			log.LogResponse(requestID, http.StatusOK, duration, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// Not an API call: render the SPA shell (SSR). Browser navigations to a
		// module endpoint that is also a real page (e.g. /posts) render that page;
		// navigations to endpoints with no page (e.g. /papers) are redirected to
		// the homepage by the client-side catch-all route.
		log.LogSystemEvent("Rendering main page (SSR)", log.LogLevelInfo, map[string]interface{}{
			"request_id": requestID,
			"path":       r.URL.Path,
		})

		duration := time.Since(startTime).Seconds() * 1000
		log.LogResponse(requestID, http.StatusOK, duration, userID)
		controllers.Index(w, r)
	})
}

// Example: Extract module name from path (customize as needed)
func getModuleNameFromPath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

// HasModuleRight reports whether the user may access the given module. Some
// modules (user administration) require an authenticated session; public
// modules such as posts remain open to anonymous visitors.
func HasModuleRight(userID int, moduleName, requiredRight string) bool {
	// Modules that must never be exposed to unauthenticated visitors.
	protected := map[string]bool{
		"users":         true,
		"user_groups":   true,
		"module_rights": true,
	}

	if protected[moduleName] {
		return userID > 0
	}

	// TODO: for finer-grained control, look up per-user rights here.
	return true
}

// getSessionID extracts or generates a session ID from the request
func getSessionID(r *http.Request) string {
	// Try to get session ID from cookie
	if cookie, err := r.Cookie("session_id"); err == nil {
		return cookie.Value
	}

	// Try to get from custom header
	if sessionID := r.Header.Get("X-Session-ID"); sessionID != "" {
		return sessionID
	}

	// Generate a basic session ID from IP and UserAgent
	return r.RemoteAddr + "_" + r.UserAgent()
}
