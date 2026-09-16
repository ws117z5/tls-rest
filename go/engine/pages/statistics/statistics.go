// Package statistics is the admin-only analytics page: access_log counts and
// breakdowns (module/page/country/ip/user-agent), filterable by any of those
// plus a time range, plus a time-range-only new-user count.
package statistics

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"tls-rest/go/engine/controllers/db/pgdb"
	"tls-rest/go/engine/controllers/functions"
	"tls-rest/go/engine/controllers/module"
)

// filters holds the parsed, optional query parameters for one request; a
// zero field means "don't filter on this" — country/module/path default to
// unfiltered (see accesslog.CountryForIP), and an unset from/to means all-time.
type filters struct {
	module    string
	path      string
	country   string
	ip        string
	userAgent string
	from      time.Time
	to        time.Time
}

func parseFilters(r *http.Request) filters {
	q := r.URL.Query()
	f := filters{
		module:    strings.TrimSpace(q.Get("module")),
		path:      strings.TrimSpace(q.Get("path")),
		country:   strings.TrimSpace(q.Get("country")),
		ip:        strings.TrimSpace(q.Get("ip")),
		userAgent: strings.TrimSpace(q.Get("user_agent")),
	}
	if v := q.Get("from"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			f.from = t
		}
	}
	if v := q.Get("to"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			f.to = t.Add(24 * time.Hour) // inclusive of the whole "to" day
		}
	}
	return f
}

// accessLogWhere builds the module/page/country/time WHERE clause (without
// the word WHERE — "" when nothing is set) and its bind args, for access_log.
func (f filters) accessLogWhere() (string, []interface{}) {
	var conds []string
	var args []interface{}
	add := func(cond string, arg interface{}) {
		conds = append(conds, fmt.Sprintf(cond, len(args)+1))
		args = append(args, arg)
	}
	if f.module != "" {
		add("module = $%d", f.module)
	}
	if f.path != "" {
		add("path LIKE $%d", f.path+"%")
	}
	if f.country != "" {
		add("country = $%d", f.country)
	}
	if f.ip != "" {
		add("ip = $%d", f.ip)
	}
	if f.userAgent != "" {
		add("user_agent LIKE $%d", "%"+f.userAgent+"%")
	}
	if !f.from.IsZero() {
		add("ts >= $%d", f.from)
	}
	if !f.to.IsZero() {
		add("ts < $%d", f.to)
	}
	if len(conds) == 0 {
		return "", nil
	}
	return "WHERE " + strings.Join(conds, " AND "), args
}

// usersWhere builds the time-range WHERE clause for the new-users count —
// module/page/country don't apply, since users carries no signup IP.
func (f filters) usersWhere() (string, []interface{}) {
	var conds []string
	var args []interface{}
	if !f.from.IsZero() {
		conds = append(conds, fmt.Sprintf("created >= $%d", len(args)+1))
		args = append(args, f.from)
	}
	if !f.to.IsZero() {
		conds = append(conds, fmt.Sprintf("created < $%d", len(args)+1))
		args = append(args, f.to)
	}
	if len(conds) == 0 {
		return "", nil
	}
	return "WHERE " + strings.Join(conds, " AND "), args
}

// scalarCount runs a "SELECT COUNT(*) AS n ..." query and returns n (0 on error).
func scalarCount(db *pgdb.Db, query string, args []interface{}) int {
	row, err := db.GetOne(query, args...)
	if err != nil || row == nil {
		return 0
	}
	return functions.Coerce[int](row["n"])
}

// breakdown returns the top 20 non-null values of column within where/args
// (already filtered access_log rows), most frequent first.
func breakdown(db *pgdb.Db, column, where string, args []interface{}) []map[string]interface{} {
	notNull := column + " IS NOT NULL"
	if where == "" {
		where = "WHERE " + notNull
	} else {
		where += " AND " + notNull
	}
	query := fmt.Sprintf(
		"SELECT %s AS value, COUNT(*) AS count FROM access_log %s GROUP BY %s ORDER BY count DESC LIMIT 20",
		column, where, column,
	)
	rows, err := db.GetAll(query, args...)
	if err != nil {
		return nil
	}
	return rows
}

// Stats handles GET /api/statistics?module=&path=&country=&ip=&user_agent=&from=&to=.
// ADMIN ONLY, enforced by Page's RequiresAdmin (see module.PageAbstract.guard).
// from/to are "YYYY-MM-DD"; omitted means unbounded/all-time.
func Stats(w http.ResponseWriter, r *http.Request) {
	db, err := pgdb.GetInstance()
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "database unavailable")
		return
	}

	f := parseFilters(r)
	where, args := f.accessLogWhere()

	blockedWhere := "WHERE blocked = true"
	if where != "" {
		blockedWhere = where + " AND blocked = true"
	}

	uwhere, uargs := f.usersWhere()

	sessionsWhere := "WHERE session_id IS NOT NULL"
	if where != "" {
		sessionsWhere = where + " AND session_id IS NOT NULL"
	}

	functions.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"total_requests":   scalarCount(db, "SELECT COUNT(*) AS n FROM access_log "+where, args),
		"blocked_requests": scalarCount(db, "SELECT COUNT(*) AS n FROM access_log "+blockedWhere, args),
		"unique_sessions":  scalarCount(db, "SELECT COUNT(DISTINCT session_id) AS n FROM access_log "+sessionsWhere, args),
		"new_users":        scalarCount(db, "SELECT COUNT(*) AS n FROM users "+uwhere, uargs),
		"by_module":        breakdown(db, "module", where, args),
		"by_path":          breakdown(db, "path", where, args),
		"by_country":       breakdown(db, "country", where, args),
		"by_ip":            breakdown(db, "ip", where, args),
		"by_user_agent":    breakdown(db, "user_agent", where, args),
	})
}

// Page self-registers the admin-only statistics endpoint and menu entry.
var Page = &module.PageAbstract{
	ID:            "statistics",
	Name:          "Statistics",
	Icon:          "statistics",
	Submenu:       "engine",
	RequiresAuth:  true,
	RequiresAdmin: true,
	Routes: []module.PageRoute{
		{Path: "/api/statistics", Methods: []string{"GET"}, Handler: Stats},
		{Path: "/api/statistics/live", Methods: []string{"GET"}, Handler: Live},
		{Path: "/api/statistics/geoip/status", Methods: []string{"GET"}, Handler: GeoIPStatus},
		{Path: "/api/statistics/geoip/backfill", Methods: []string{"POST"}, Handler: GeoIPBackfill},
	},
}

func Init() { Page.Initialize() }
