// Package sitemap defines how a module lists its publicly viewable records for /sitemap.xml (served by go/pages/seo).
package sitemap

import (
	"context"
	"time"
)

// Entry is one public URL path (e.g. "/posts/12") and when it last changed.
type Entry struct {
	Path    string
	Lastmod time.Time
}

// Source is implemented by a module (on its struct) to expose records a guest may view; the sitemap includes them only if guests may view the module.
type Source interface {
	SitemapEntries(ctx context.Context) []Entry
}
