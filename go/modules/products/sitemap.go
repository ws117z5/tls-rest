package products

import (
	"context"
	"fmt"
	"time"

	"tls-rest/go/engine/controllers/db/pgdb"
	"tls-rest/go/engine/controllers/sitemap"
)

// SitemapEntries lists active, public-access products.
func (p *Products) SitemapEntries(ctx context.Context) []sitemap.Entry {
	db, err := pgdb.GetInstanceCtx(ctx)
	if err != nil {
		return nil
	}
	rows, err := db.GetAll("SELECT id, updated FROM products WHERE active = true AND access <= 0 ORDER BY updated DESC LIMIT 5000")
	if err != nil {
		return nil
	}
	out := make([]sitemap.Entry, 0, len(rows))
	for _, r := range rows {
		e := sitemap.Entry{Path: fmt.Sprintf("/products/%v", r["id"])}
		if t, ok := r["updated"].(time.Time); ok {
			e.Lastmod = t
		}
		out = append(out, e)
	}
	return out
}
