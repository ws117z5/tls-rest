// Package seo serves GET /robots.txt and GET /sitemap.xml, generated from what an anonymous visitor may actually see.
package seo

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"tls-rest/go/engine/controllers/auth"
	"tls-rest/go/engine/controllers/httpx"
	"tls-rest/go/engine/controllers/module"
	"tls-rest/go/engine/controllers/sitemap"

	"github.com/gorilla/mux"
)

const (
	cacheTTL   = 10 * time.Minute
	maxEntries = 50000 // sitemap protocol limit per file
)

var skipPages = map[string]bool{"login": true}

type urlEntry struct {
	Loc     string `xml:"loc"`
	Lastmod string `xml:"lastmod,omitempty"`
}

type urlSet struct {
	XMLName xml.Name   `xml:"http://www.sitemaps.org/schemas/sitemap/0.9 urlset"`
	URLs    []urlEntry `xml:"url"`
}

var (
	mu        sync.Mutex
	cached    []byte
	cachedFor string
	cachedAt  time.Time
)

func robots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, "User-agent: *\nDisallow: /api/\nDisallow: /login\nDisallow: /users/Auth\n\nSitemap: %s/sitemap.xml\n", httpx.BaseURL(r))
}

// entries collects the paths an anonymous visitor may open: guest-listable modules, guest-viewable pages, and records modules expose via sitemap.Source.
func entries(ctx context.Context) []sitemap.Entry {
	rights := auth.ResolveModuleModeRights(ctx, 0)
	out := []sitemap.Entry{{Path: "/"}}

	ids := make([]string, 0, len(module.RegisteredModules))
	for id := range module.RegisteredModules {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		m := module.RegisteredModules[id]
		if m.IsHidden() || !auth.HasMode(rights, id, auth.MODE_LIST, false) {
			continue
		}
		out = append(out, sitemap.Entry{Path: "/" + id})
		if src, ok := m.(sitemap.Source); ok && auth.HasMode(rights, id, auth.MODE_VIEW, false) {
			out = append(out, src.SitemapEntries(ctx)...)
		}
	}

	for _, p := range module.RegisteredPageMenu() {
		if skipPages[p.ID] || !auth.HasPageMode(rights, p.ID, auth.MODE_VIEW, false) {
			continue
		}
		out = append(out, sitemap.Entry{Path: "/" + p.ID})
	}
	return out
}

func build(ctx context.Context, base string) ([]byte, error) {
	set := urlSet{}
	for _, e := range entries(ctx) {
		if len(set.URLs) >= maxEntries {
			break
		}
		u := urlEntry{Loc: base + e.Path}
		if !e.Lastmod.IsZero() {
			u.Lastmod = e.Lastmod.UTC().Format("2006-01-02")
		}
		set.URLs = append(set.URLs, u)
	}
	body, err := xml.MarshalIndent(set, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), body...), nil
}

func sitemapXML(w http.ResponseWriter, r *http.Request) {
	base := strings.TrimRight(httpx.BaseURL(r), "/")

	mu.Lock()
	defer mu.Unlock()
	if cached == nil || cachedFor != base || time.Since(cachedAt) > cacheTTL {
		body, err := build(r.Context(), base)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		cached, cachedFor, cachedAt = body, base, time.Now()
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	_, _ = w.Write(cached)
}

func init() {
	module.AddRouteRegistrar(func(router *mux.Router) {
		router.HandleFunc("/robots.txt", robots).Methods(http.MethodGet)
		router.HandleFunc("/sitemap.xml", sitemapXML).Methods(http.MethodGet)
	})
}
