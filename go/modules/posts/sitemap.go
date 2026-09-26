package posts

import (
	"context"
	"fmt"
	"time"

	"tls-rest/go/engine/controllers/auth"
	"tls-rest/go/engine/controllers/db/pgdb"
	"tls-rest/go/engine/controllers/sitemap"
)

// SitemapEntries lists posts shared with the guest group; anything else is private to its author and admins.
func (p *Posts) SitemapEntries(ctx context.Context) []sitemap.Entry {
	db, err := pgdb.GetInstanceCtx(ctx)
	if err != nil {
		return nil
	}
	rows, err := db.GetAll(
		"SELECT id, updated FROM posts WHERE access <= 0 AND visible_groups @> $1::jsonb ORDER BY updated DESC LIMIT 5000",
		fmt.Sprintf("[%d]", auth.GuestGroupID),
	)
	if err != nil {
		return nil
	}
	out := make([]sitemap.Entry, 0, len(rows))
	for _, r := range rows {
		e := sitemap.Entry{Path: fmt.Sprintf("/posts/%v", r["id"])}
		if t, ok := r["updated"].(time.Time); ok {
			e.Lastmod = t
		}
		out = append(out, e)
	}
	return out
}
