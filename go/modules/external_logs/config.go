package external_logs

import (
	"net/http"

	"tls-rest/go/engine/controllers/db/cache"
	"tls-rest/go/engine/controllers/db/pgdb"
	"tls-rest/go/engine/controllers/functions"
)

// configHandler handles GET /external_logs/config?app=words: whether the
// named app's clients should send logs at all, read from
// external_config.send_logs (engine/modules/externalconfig) — the Android
// apps call this once at startup (RemoteLog.init) and skip every send
// locally when it comes back false, rather than submitting logs the server
// would just accept and never want. No external_config row for the app, or
// no "app" param, defaults to enabled (today's behavior for apps that
// predate this flag).
func (m *ExternalLogs) configHandler(w http.ResponseWriter, r *http.Request) {
	s := cache.SessionFromContext(r.Context())
	if s == nil || s.UserID <= 0 {
		functions.JSONError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	appModule := r.URL.Query().Get("app")
	enabled := true
	if appModule != "" {
		db, err := pgdb.GetInstanceCtx(r.Context())
		if err != nil {
			functions.JSONError(w, http.StatusInternalServerError, "database unavailable")
			return
		}
		row, err := db.GetOne(`SELECT send_logs FROM external_config WHERE app_module = $1 LIMIT 1`, appModule)
		if err != nil {
			functions.JSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if row != nil && row["send_logs"] != nil {
			enabled = functions.Truthy(row["send_logs"])
		}
	}

	functions.WriteJSON(w, http.StatusOK, map[string]interface{}{"enabled": enabled})
}
