// Package actions is the admin-only page over the actions registry
// (engine/controllers/actions): list what's registered, trigger one now, or
// set/clear its recurring interval.
package actions

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"tls-rest/go/app"
	actionsctl "tls-rest/go/engine/controllers/actions"
	"tls-rest/go/engine/controllers/db/cache"
	"tls-rest/go/engine/controllers/db/pgdb"
	"tls-rest/go/engine/controllers/functions"
	"tls-rest/go/engine/controllers/module"

	"github.com/gorilla/mux"
)

// List handles GET /api/actions. ADMIN ONLY, enforced by Page's RequiresAdmin
// (see module.PageAbstract.guard).
func List(w http.ResponseWriter, r *http.Request) {
	all := actionsctl.All()
	out := make([]actionsctl.Snapshot, 0, len(all))
	for _, a := range all {
		out = append(out, a.Snapshot())
	}
	functions.WriteJSON(w, http.StatusOK, map[string]interface{}{"actions": out})
}

// Run handles POST /api/actions/{id}/run {params: {...}}. ADMIN ONLY,
// enforced by Page's RequiresAdmin (see module.PageAbstract.guard). params
// is optional — omitted/empty means "run with whatever last ran" (see
// Action.RunNow). admin_uuid/server are filled in from the current request
// whenever the caller didn't already set them — the Test rig action
// (go/engine/pages/actions/jobs/rig.go) needs both to provision test
// accounts and friend them with whoever is actually running the test,
// without the admin having to look either value up by hand.
func Run(w http.ResponseWriter, r *http.Request) {
	a, ok := actionsctl.Get(mux.Vars(r)["id"])
	if !ok {
		functions.JSONError(w, http.StatusNotFound, "unknown action")
		return
	}
	var body struct {
		Params map[string]interface{} `json:"params"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.Params == nil {
		body.Params = map[string]interface{}{}
	}
	fillRequestDefaults(r, body.Params)
	result, err := a.RunNow(body.Params)
	resp := map[string]interface{}{"result": result}
	if err != nil {
		resp["error"] = err.Error()
	}
	functions.WriteJSON(w, http.StatusOK, resp)
}

func fillRequestDefaults(r *http.Request, params map[string]interface{}) {
	if _, ok := params["admin_uuid"]; !ok {
		if s := cache.SessionFromContext(r.Context()); s != nil && s.UserID > 0 {
			if db, err := pgdb.GetInstanceCtx(r.Context()); err == nil {
				if row, err := db.GetOne("SELECT uuid FROM users WHERE id = $1", s.UserID); err == nil && row != nil {
					if uuid := functions.Coerce[string](row["uuid"]); uuid != "" {
						params["admin_uuid"] = uuid
					}
				}
			}
		}
	}
	if _, ok := params["server"]; !ok {
		params["server"] = requestBaseURL(r)
	}
}

// requestBaseURL reconstructs this request's own scheme+host — the rig
// connects back to wherever it's actually running, not a hardcoded URL.
// The app is HTTPS-only (see the Dockerfile's own comment on :8443) and may
// sit behind a reverse proxy (see docker-compose.yml's Traefik labels), so a
// proxy-set X-Forwarded-* header wins when present.
func requestBaseURL(r *http.Request) string {
	scheme := "https"
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	} else if r.TLS == nil {
		scheme = "http"
	}
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	return scheme + "://" + host
}

// Output handles GET /api/actions/{id}/output?from=N: a Command action's terminal output since offset N.
func Output(w http.ResponseWriter, r *http.Request) {
	a, ok := actionsctl.Get(mux.Vars(r)["id"])
	if !ok {
		functions.JSONError(w, http.StatusNotFound, "unknown action")
		return
	}
	from, _ := strconv.Atoi(r.URL.Query().Get("from"))
	text, next, running := a.Output(from)
	functions.WriteJSON(w, http.StatusOK, map[string]interface{}{"text": text, "next": next, "running": running})
}

// Input handles POST /api/actions/{id}/input {text, enter}: types into a running Command action's terminal.
func Input(w http.ResponseWriter, r *http.Request) {
	a, ok := actionsctl.Get(mux.Vars(r)["id"])
	if !ok {
		functions.JSONError(w, http.StatusNotFound, "unknown action")
		return
	}
	var body struct {
		Text  string `json:"text"`
		Enter bool   `json:"enter"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if err := a.Input(body.Text, body.Enter); err != nil {
		functions.JSONError(w, http.StatusConflict, err.Error())
		return
	}
	functions.WriteJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

// Stop handles POST /api/actions/{id}/stop: terminates a running Command action.
func Stop(w http.ResponseWriter, r *http.Request) {
	a, ok := actionsctl.Get(mux.Vars(r)["id"])
	if !ok {
		functions.JSONError(w, http.StatusNotFound, "unknown action")
		return
	}
	a.Stop()
	functions.WriteJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

// Schedule handles POST /api/actions/{id}/schedule {interval_seconds}. ADMIN
// ONLY, enforced by Page's RequiresAdmin (see module.PageAbstract.guard). 0
// (or omitted) cancels any existing schedule.
func Schedule(w http.ResponseWriter, r *http.Request) {
	a, ok := actionsctl.Get(mux.Vars(r)["id"])
	if !ok {
		functions.JSONError(w, http.StatusNotFound, "unknown action")
		return
	}
	var body struct {
		IntervalSeconds int `json:"interval_seconds"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	a.SetSchedule(time.Duration(body.IntervalSeconds) * time.Second)
	functions.WriteJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

// Page self-registers the admin-only actions endpoints and menu entry.
var Page = &module.PageAbstract{
	ID:            "actions",
	Name:          "Actions",
	Icon:          "config",
	Submenu:       "engine",
	RequiresAuth:  true,
	RequiresAdmin: true,
	Routes: []module.PageRoute{
		{Path: "/api/actions", Methods: []string{"GET"}, Handler: List},
		{Path: "/api/actions/{id}/run", Methods: []string{"POST"}, Handler: Run},
		{Path: "/api/actions/{id}/output", Methods: []string{"GET"}, Handler: Output},
		{Path: "/api/actions/{id}/input", Methods: []string{"POST"}, Handler: Input},
		{Path: "/api/actions/{id}/stop", Methods: []string{"POST"}, Handler: Stop},
		{Path: "/api/actions/{id}/schedule", Methods: []string{"POST"}, Handler: Schedule},
	},
}

func init() {
	app.RegisterPage(Page)
}
