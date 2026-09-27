// Package html is a shared compiled-HTML content store, referenced by id (e.g. posts.go's html_id).
package html

import (
	"bytes"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/yuin/goldmark"

	"tls-rest/go/app"
	"tls-rest/go/engine/controllers/db/pgdb"
	"tls-rest/go/engine/controllers/field"
	"tls-rest/go/engine/controllers/functions"
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
		field.NewField("slug", field.TYPE_STRING, false).WithLabel("Slug").
			WithDescription("Publishes this row at /api/html/<slug> for pure-JS pages (e.g. legal.terms); leave empty to keep it private"),
		field.NewField("compiled_html", field.TYPE_HTML, true).WithLabel("HTML"),
	},
	DefaultPermission:    module.PERMISSION_DENY,
	DefaultPermissionSet: true,
	Rights:               make(map[int]int),
	CustomRoutes: []module.CustomRoute{
		{Path: "/api/html/{slug}", Methods: []string{http.MethodGet}, Handler: bySlug, Absolute: true},
	},
}

// bySlug serves a published html row (one with a slug) to anyone; rows without a slug, such as post content, are never reachable here.
func bySlug(w http.ResponseWriter, r *http.Request) {
	db, err := pgdb.GetInstanceCtx(r.Context())
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "database unavailable")
		return
	}
	row, err := db.GetOne("SELECT slug, compiled_html FROM html WHERE slug = $1 AND slug <> ''", mux.Vars(r)["slug"])
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if row == nil {
		functions.JSONError(w, http.StatusNotFound, "not found")
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=300")
	functions.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"slug": functions.Coerce[string](row["slug"]),
		"html": functions.Coerce[string](row["compiled_html"]),
	})
}

func init() {
	app.RegisterModule(Module, "html")
}
