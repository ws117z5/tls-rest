package profile

import (
	"net/http"

	"tls-rest/go/engine/controllers/auth"
	"tls-rest/go/engine/controllers/db/cache"
	"tls-rest/go/engine/controllers/functions"
)

// handleGenerateClaudeToken handles POST /api/profile/claude-token: mints a
// bearer token (same mechanism as a mobile login, auth.IssueToken) for the
// caller's local Claude Code "Stop" hook to use when posting to
// go/modules/claudenotify. Shown once in the Profile UI — there's no way to
// recover it later, same as every other bearer token this app issues; the
// user generates a new one (and updates the hook's token file) if it's lost.
func handleGenerateClaudeToken(w http.ResponseWriter, r *http.Request) {
	s := cache.SessionFromContext(r.Context())
	if s == nil || s.UserID <= 0 {
		functions.JSONError(w, http.StatusUnauthorized, "no session")
		return
	}
	token, expire, err := auth.IssueToken(r.Context(), s.UserID, s.Username)
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "could not issue token")
		return
	}
	functions.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"token":   token,
		"expires": expire.Unix(),
	})
}
