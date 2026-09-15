package statistics

import (
	"fmt"
	"net/http"

	"tls-rest/go/engine/controllers/accesslog"
	"tls-rest/go/engine/controllers/db/cache"
	"tls-rest/go/engine/controllers/db/pgdb"
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

	db, err := pgdb.GetInstance()
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "database unavailable")
		return
	}

	where, args := parseFilters(r).accessLogWhere()
	cond := "country IS NULL AND ip IS NOT NULL"
	if where == "" {
		where = "WHERE " + cond
	} else {
		where += " AND " + cond
	}
	rows, err := db.GetAll(fmt.Sprintf("SELECT id, ip FROM access_log %s LIMIT %d", where, geoipBackfillLimit), args...)
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	resolved := 0
	for _, row := range rows {
		id := functions.Coerce[int64](row["id"])
		ip := functions.Coerce[string](row["ip"])
		country := accesslog.CountryForIP(ip)
		if country == "" {
			continue
		}
		if _, err := db.Exec("UPDATE access_log SET country = $1 WHERE id = $2", country, id); err == nil {
			resolved++
		}
	}

	functions.WriteJSON(w, http.StatusOK, map[string]interface{}{"loaded": true, "resolved": resolved, "checked": len(rows)})
}
