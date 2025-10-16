package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/ws117z5/tls-rest/go/controllers"
	. "github.com/ws117z5/tls-rest/go/lib/auth"
	"github.com/ws117z5/tls-rest/go/lib/db/cache"
	"github.com/ws117z5/tls-rest/go/lib/log"
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

	// If explicitly marked as API
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

	// Auth endpoints are API calls
	if strings.HasPrefix(uri, "/users/Auth") {
		return true
	}

	// Papers API endpoints
	if strings.HasPrefix(uri, "/papers") {
		return true
	}

	// Module API endpoints - check multiple indicators for API calls
	acceptHeader := r.Header.Get("Accept")
	userAgent := r.Header.Get("User-Agent")
	xRequestedWith := r.Header.Get("X-Requested-With")

	// Check if this is likely an API call
	isLikelyAPI := strings.Contains(acceptHeader, "application/json") ||
		xRequestedWith == "XMLHttpRequest" ||
		strings.Contains(userAgent, "fetch") ||
		r.Method != "GET" // POST, PUT, DELETE are typically API calls

	if isLikelyAPI {
		// These are actual API calls from JavaScript or API clients
		if strings.HasPrefix(uri, "/posts") ||
			strings.HasPrefix(uri, "/users") ||
			strings.HasPrefix(uri, "/user_groups") ||
			strings.HasPrefix(uri, "/module_rights") {
			return true
		}
	}

	// Home/root is NOT an API call
	if uri == "/" {
		return false
	}

	// Default: render page for all other requests (SSR)
	return false
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

		// Log system event for page render
		log.LogSystemEvent("Rendering main page (SSR)", log.LogLevelInfo, map[string]interface{}{
			"request_id": requestID,
			"path":       r.URL.Path,
		})

		// Not an API call: always render the main index page (SSR)
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

// Example: Check if user has rights for the module (implement DB/cache logic)
func HasModuleRight(userID int, moduleName, requiredRight string) bool {
	// TODO: Query DB or cache for user's rights for this module
	// Return true if user has the required right, false otherwise
	return true // placeholder: always allow
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
