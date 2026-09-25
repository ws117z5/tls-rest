// Transformer Studio tool page (js/src/pages/transformerstudio), registered for rights admin.
package transformerstudio

import (
	"tls-rest/go/app"
	"tls-rest/go/engine/controllers/module"
)

var Page = &module.PageAbstract{
	ID:           "transformer-studio",
	Name:         "Transformer Studio",
	Icon:         "statistics",
	Submenu:      "tools",
	RequiresAuth: true,
}

func init() { app.RegisterPage(Page) }
