package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"tls-rest/go/controllers"
	module "tls-rest/go/engine"
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

	// Image bytes are served by a real handler (ServeByRef) as a browser GET
	// (e.g. <img src="/image/{guid}.{ext}"> or direct navigation). Without this
	// it looks like a page navigation and the middleware would render the SPA
	// shell instead of the image — which then client-redirects home.
	if strings.HasPrefix(uri, "/image/") {
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

// isModuleEndpoint reports whether a path is a JSON data endpoint (a module CRUD
// route, page endpoint, or feature route) rather than an SPA page. The set is
// self-registered by modules/pages/features in the engine — see
// module.RegisterEndpointPrefix — so adding one never requires editing this.
func isModuleEndpoint(path string) bool {
	return module.IsRegisteredEndpoint(path)
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
			// --- Module rights check (per-mode, group-resolved) ---
			allowed, moduleName, action := authorizeAPIRequest(ci, r)

			if !allowed {
				if r.URL.Path == "/login" || r.URL.Path == "/" {
					duration := time.Since(startTime).Seconds() * 1000
					log.LogResponse(requestID, http.StatusOK, duration, userID)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
				log.LogAuthEvent("authorization_failed", "User lacks required module rights", userID, sessionID, false, map[string]interface{}{
					"module":     moduleName,
					"action":     action,
					"request_id": requestID,
				})

				duration := time.Since(startTime).Seconds() * 1000
				log.LogResponse(requestID, http.StatusUnauthorized, duration, userID)
				controllers.Error(w, r.WithContext(ctx), http.StatusUnauthorized)
				return
			}
			// --- End module rights check ---

			log.LogModuleEvent(moduleName, action, "Module access granted", userID, sessionID, map[string]interface{}{
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

// authorizeAPIRequest decides whether an API request may proceed, using the
// per-mode rights resolved onto the session. It returns the decision plus the
// module and action for logging.
//
//   - "/" and "/login" are always allowed (they render the shell / login).
//   - /api/modules/{id}/fieldset requires VIEW on {id}.
//   - /api/modules (the module list) is self-filtering, so it is allowed.
//   - any /api/*images* endpoint requires an authenticated user.
//   - a request to a rights-managed module (one with a registered default)
//     is gated by the mode implied by its HTTP method (see apiActionMode).
//   - everything else (papers, metrics, log, …) keeps the permissive legacy
//     behaviour via HasModuleRight.
func authorizeAPIRequest(ci *cache.Session, r *http.Request) (allowed bool, module string, action string) {
	path := r.URL.Path

	if path == "/" || path == "/login" {
		return true, "", "page"
	}

	// Public authentication flows: OAuth redirect/callback and the password
	// login/logout/register endpoints. These must be reachable while anonymous
	// and must NOT be gated as the "users" module.
	if strings.HasPrefix(path, "/users/Auth") {
		return true, "auth", "oauth"
	}

	// Public image serving (GET /image/{guid}.{ext}). Byte serving is allowed
	// here; ServeByRef itself enforces per-record access via CanViewRecord, so a
	// restricted image still 404s for users who can't view its owning record.
	if strings.HasPrefix(path, "/image/") {
		return true, "images", "image"
	}
	if path == "/api/login" || path == "/api/logout" || path == "/api/register" {
		return true, "auth", "login"
	}

	if strings.HasPrefix(path, "/api/") {
		// Fieldset schema for a specific module: needs read (VIEW) on it.
		if strings.HasPrefix(path, "/api/modules/") && strings.HasSuffix(path, "/fieldset") {
			mid := strings.TrimSuffix(strings.TrimPrefix(path, "/api/modules/"), "/fieldset")
			mid = strings.Trim(mid, "/")
			return HasMode(ci.ModuleModes, mid, MODE_VIEW, ci.IsAdmin), mid, "fieldset"
		}
		// The module list endpoint filters itself per user.
		if path == "/api/modules" {
			return true, "modules", "list"
		}
		// Image endpoints: uploading/processing requires an authenticated user;
		// serving image bytes is public so a public record's images render for
		// anyone.
		if strings.Contains(path, "/images") {
			if r.Method == http.MethodPost || strings.HasSuffix(path, "/process") {
				return ci.UserID > 0, "images", "image"
			}
			return true, "images", "image"
		}
		return true, getModuleNameFromPath(path), "api"
	}

	module = getModuleNameFromPath(path)

	// Rights-managed module: gate by the mode implied by the method.
	if _, managed := ModuleDefaults()[module]; managed {
		mode := apiActionMode(r.Method, path, module)
		return HasMode(ci.ModuleModes, module, mode, ci.IsAdmin), module, apiActionName(mode)
	}

	// Non-managed route (feature endpoints): keep the legacy permissive check.
	return HasModuleRight(ci.UserID, module, "view"), module, "legacy"
}

// apiActionMode maps an HTTP method (and whether the path targets a single
// record) to the access mode it requires.
func apiActionMode(method, path, module string) int {
	// A trailing /{id} after the module segment means a single-record op.
	rest := strings.Trim(strings.TrimPrefix(strings.Trim(path, "/"), module), "/")
	hasID := rest != ""

	switch method {
	case http.MethodPost:
		return MODE_CREATE
	case http.MethodPut, http.MethodPatch:
		return MODE_EDIT
	case http.MethodDelete:
		return MODE_DELETE
	default: // GET / HEAD
		if hasID {
			return MODE_VIEW
		}
		return MODE_LIST
	}
}

func apiActionName(mode int) string {
	switch mode {
	case MODE_LIST:
		return "list"
	case MODE_VIEW:
		return "view"
	case MODE_CREATE:
		return "create"
	case MODE_EDIT:
		return "edit"
	case MODE_DELETE:
		return "delete"
	default:
		return "access"
	}
}

// HasModuleRight reports whether the user may access the given module. Some
// modules (user administration) require an authenticated session; public
// modules such as posts remain open to anonymous visitors.
func HasModuleRight(userID int, moduleName, requiredRight string) bool {
	// Modules that must never be exposed to unauthenticated visitors.
	protected := map[string]bool{
		"users":             true,
		"user_groups":       true,
		"user_group_rights": true,
		"user_rights":       true,
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
