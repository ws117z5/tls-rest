// Package markdowntool registers the client-only Markdown Renderer tool page (js/src/pages/markdowntool) for rights admin + real RequiresAuth enforcement.
package markdowntool

import (
	"tls-rest/go/app"
	"tls-rest/go/engine/controllers/module"
)

var Page = &module.PageAbstract{
	ID:           "markdown-tool",
	Name:         "Markdown Renderer",
	Icon:         "markdown",
	Submenu:      "tools",
	RequiresAuth: true,
}

func init() { app.RegisterPage(Page) }
