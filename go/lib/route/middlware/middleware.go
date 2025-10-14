package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/ws117z5/tls-rest/go/controllers"
	. "github.com/ws117z5/tls-rest/go/lib/auth"
	"github.com/ws117z5/tls-rest/go/lib/db/cache"
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

	// Home/root is NOT an API call
	if uri == "/" {
		return false
	}

	// Default: treat as API if it's not a static asset or root
	return true
}

// Middleware dlv fails here
// TODO reinstall llvm on osx, test dlv for errors, for now testing only in
func (amw *AuthenticationMiddleware) Middleware(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check or set session cookie
		ci := ManageSession(w, r)
		ctx := context.WithValue(r.Context(), SESSION_KEY, ci)

		//fmt.Println("Session ID:", cookie.Value, "User ID:", userID, "Authenticated:", auth)

		// Only return data for API calls, otherwise render index.html
		if !isAPICall(r) {
			controllers.Index(w, r)
			return
		}

		//no auth required for

		//home page and login
		//pages
		//posts

		// --- Module rights check ---
		moduleName := getModuleNameFromPath(r.URL.Path)
		requiredRight := "view" // or "edit", etc. -- set as needed

		// For API calls, check authentication
		if !HasModuleRight(ci.UserID, moduleName, requiredRight) {
			if r.URL.Path == "/login" || r.URL.Path == "/" {
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			controllers.Error(w, r.WithContext(ctx), http.StatusUnauthorized)
			return
		}
		// --- End module rights check ---

		// Authenticated and authorized, proceed
		next.ServeHTTP(w, r.WithContext(ctx))

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
