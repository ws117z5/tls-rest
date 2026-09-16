// Package actions is the admin-only page over the actions registry
// (engine/controllers/actions): list what's registered, trigger one now, or
// set/clear its recurring interval.
package actions

import (
	"encoding/json"
	"net/http"
	"time"

	actionsctl "tls-rest/go/engine/controllers/actions"
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

// Run handles POST /api/actions/{id}/run. ADMIN ONLY, enforced by Page's
// RequiresAdmin (see module.PageAbstract.guard).
func Run(w http.ResponseWriter, r *http.Request) {
	a, ok := actionsctl.Get(mux.Vars(r)["id"])
	if !ok {
		functions.JSONError(w, http.StatusNotFound, "unknown action")
		return
	}
	result, err := a.RunNow()
	resp := map[string]interface{}{"result": result}
	if err != nil {
		resp["error"] = err.Error()
	}
	functions.WriteJSON(w, http.StatusOK, resp)
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
		{Path: "/api/actions/{id}/schedule", Methods: []string{"POST"}, Handler: Schedule},
	},
}

func Init() {
	Page.Initialize()
	initTurnCheckAction()
}
