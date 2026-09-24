// Package arrayiterator registers the client-only Array Iterator tool page (js/src/pages/arrayiterator) for rights admin + real RequiresAuth enforcement.
package arrayiterator

import (
	"tls-rest/go/app"
	"tls-rest/go/engine/controllers/module"
)

var Page = &module.PageAbstract{
	ID:           "arrayiter",
	Name:         "Array Iterator",
	Icon:         "array-iterator",
	Submenu:      "tools",
	RequiresAuth: true,
}

func init() { app.RegisterPage(Page) }
