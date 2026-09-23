// Package console exposes the live actions console over HTTP, admin-only. It is a
// thin transport in front of the input controller (subroutine/input): the page
// module writes a command to this endpoint and reads back the output.
package console

import (
	"encoding/json"
	"net/http"

	"tls-rest/go/app"
	"tls-rest/go/engine/controllers/functions"
	"tls-rest/go/engine/controllers/module"
	"tls-rest/go/engine/controllers/subroutine/input"
)

type request struct {
	Command string `json:"command"`
}

// Run handles POST /api/console {command}. ADMIN ONLY: the console can read/alter
// caches, run queries, change rights and touch the firewall — enforced by
// Page's RequiresAdmin (see module.PageAbstract.guard).
func Run(w http.ResponseWriter, r *http.Request) {
	var req request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		functions.JSONError(w, http.StatusBadRequest, "invalid request")
		return
	}

	out, err := input.Execute(req.Command)
	resp := map[string]interface{}{"output": out}
	if err != nil {
		resp["error"] = err.Error()
	}
	functions.WriteJSON(w, http.StatusOK, resp)
}

// Page self-registers the console endpoint and an admin-only menu entry.
var Page = &module.PageAbstract{
	ID:            "console",
	Name:          "Console",
	Icon:          "console",
	Submenu:       "engine",
	RequiresAuth:  true,
	RequiresAdmin: true,
	Routes: []module.PageRoute{
		{Path: "/api/console", Methods: []string{"POST"}, Handler: Run},
	},
}

func init() { app.RegisterPage(Page) }
