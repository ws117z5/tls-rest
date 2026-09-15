package statistics

import (
	"net/http"
	"strings"

	"tls-rest/go/engine/controllers/accesslog"
	"tls-rest/go/engine/controllers/db/cache"
	"tls-rest/go/engine/controllers/functions"
)

// geoipBackfillLimit caps rows resolved per Apply click, so a huge unresolved
// backlog can't turn one click into a very long request; click Apply again to
// make further progress (it only ever touches rows still missing a country).
const geoipBackfillLimit = 5000

// GeoIPStatus handles GET /api/statistics/geoip/status. ADMIN ONLY.
func GeoIPStatus(w http.ResponseWriter, r *http.Request) {
	s := cache.SessionFromContext(r.Context())
	if s == nil || !s.IsAdmin {
		functions.JSONError(w, http.StatusForbidden, "admin only")
		return
	}
	functions.WriteJSON(w, http.StatusOK, map[string]interface{}{"loaded": accesslog.GeoIPLoaded()})
}

// GeoIPBackfill handles POST /api/statistics/geoip/backfill with the same
// query params as Stats. ADMIN ONLY. Resolves country for rows matching the
// current filters that don't have one yet, using whatever's currently loaded
// (a no-op — not an error — if nothing is). Meant to run right before Stats
// on the same Apply click, so newly-resolved rows show up in that response.
func GeoIPBackfill(w http.ResponseWriter, r *http.Request) {
	s := cache.SessionFromContext(r.Context())
	if s == nil || !s.IsAdmin {
		functions.JSONError(w, http.StatusForbidden, "admin only")
		return
	}
	if !accesslog.GeoIPLoaded() {
		functions.WriteJSON(w, http.StatusOK, map[string]interface{}{"loaded": false, "resolved": 0, "checked": 0})
		return
	}

	// accessLogWhere's fragment already reads "WHERE ..."; BackfillCountriesBatch
	// wants a bare AND-able condition, so strip that prefix back off here.
	where, args := parseFilters(r).accessLogWhere()
	where = strings.TrimPrefix(where, "WHERE ")

	resolved, checked, err := accesslog.BackfillCountriesBatch(geoipBackfillLimit, where, args)
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	functions.WriteJSON(w, http.StatusOK, map[string]interface{}{"loaded": true, "resolved": resolved, "checked": checked})
}
