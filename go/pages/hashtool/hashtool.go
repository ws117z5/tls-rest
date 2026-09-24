// Package hashtool registers the client-only Hash Generator tool page (js/src/pages/hashtool) for rights admin + real RequiresAuth enforcement.
package hashtool

import (
	"tls-rest/go/app"
	"tls-rest/go/engine/controllers/module"
)

var Page = &module.PageAbstract{
	ID:           "hash-tool",
	Name:         "Hash Generator",
	Icon:         "hash",
	Submenu:      "tools",
	RequiresAuth: true,
}

func init() { app.RegisterPage(Page) }
