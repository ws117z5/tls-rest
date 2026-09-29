// Transformer Studio tool page (js/src/pages/transformerstudio), registered for rights admin. Its data comes from a
// model server the user points the web UI at (see proxy.go), not from this app's own database.
package transformerstudio

import (
	"net/http"

	"tls-rest/go/app"
	"tls-rest/go/engine/controllers/module"
)

var Page = &module.PageAbstract{
	ID:           "transformer-studio",
	Name:         "Transformer Studio",
	Icon:         "statistics",
	Submenu:      "tools",
	RequiresAuth: true,
	Routes: []module.PageRoute{
		{Path: "/api/transformer-studio/proxy", Methods: []string{http.MethodPost}, Handler: proxyRequest},
		{Path: "/api/transformer-studio/enhance-video", Methods: []string{http.MethodPost}, Handler: proxyEnhanceVideo},
		{Path: "/api/transformer-studio/events", Methods: []string{http.MethodGet}, Handler: proxyEvents},
	},
}

func init() { app.RegisterPage(Page) }
