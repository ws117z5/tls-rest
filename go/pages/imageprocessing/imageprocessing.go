// Package imageprocessing registers the client-only ImageProcessing tool page (js/src/pages/imageprocessing) for rights admin + real RequiresAuth enforcement.
package imageprocessing

import (
	"tls-rest/go/app"
	"tls-rest/go/engine/controllers/module"
)

var Page = &module.PageAbstract{
	ID:           "imageproc",
	Name:         "ImageProcessing",
	Icon:         "image-processing",
	Submenu:      "tools",
	RequiresAuth: true,
}

func init() { app.RegisterPage(Page) }
