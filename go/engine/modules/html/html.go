// Package html is a shared compiled-HTML content store, referenced by id (e.g. posts.go's html_id).
package html

import (
	"bytes"
	"strings"

	"github.com/yuin/goldmark"

	"tls-rest/go/app"
	"tls-rest/go/engine/controllers/field"
	"tls-rest/go/engine/controllers/module"
)

// RenderMarkdown compiles markdown to HTML. goldmark's default mode escapes
// raw HTML found in the source rather than passing it through, so the result
// is safe to render with dangerouslySetInnerHTML on the frontend. :::graph
// blocks are rendered separately (renderGraphBlocks), since goldmark has no
// concept of that directive.
func RenderMarkdown(src string) string {
	stripped, graphs := renderGraphBlocks(src)
	var buf bytes.Buffer
	if err := goldmark.Convert([]byte(stripped), &buf); err != nil {
		return ""
	}
	out := buf.String()
	for placeholder, rendered := range graphs {
		out = strings.ReplaceAll(out, placeholder, rendered)
	}
	return out
}

// Module: other packages InsertRow/UpdateRow against this directly, e.g. posts.go's AfterFieldset.
var Module = &module.ModuleAbstract[interface{}]{
	ID:      "html",
	Name:    "HTML Content",
	Submenu: "engine",
	Fields: []field.Field{
		field.NewField("compiled_html", field.TYPE_HTML, true).WithLabel("HTML"),
	},
	DefaultPermission:    module.PERMISSION_DENY,
	DefaultPermissionSet: true,
	Rights:               make(map[int]int),
}

func init() {
	app.RegisterModule(Module, "html")
}
