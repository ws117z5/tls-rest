// Package graph registers the client-only Graphs tool page (js/src/pages/graph) for rights admin + real RequiresAuth enforcement.
package graph

import (
	"tls-rest/go/app"
	"tls-rest/go/engine/controllers/module"
)

var Page = &module.PageAbstract{
	ID:           "graph",
	Name:         "Graphs",
	Icon:         "graphs",
	Submenu:      "tools",
	RequiresAuth: true,
}

func init() { app.RegisterPage(Page) }
