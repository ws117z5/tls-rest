package posts

import (
	. "tls-rest/go/engine/controllers/field"
)

// filters declares the filters accepted by GET /posts (list mode).
//
// Each filter is an ordinary fieldset Field: its Name is the query parameter the
// client sends, its Type drives value casting, and the operator helper
// (Equals/Contains/GreaterOrEqual/...) decides how it becomes a SQL WHERE clause.
// See go/engine/filter.go for the available helpers.
//
// Examples:
//
//	GET /posts?title=hello            -> WHERE title ILIKE '%hello%'
//	GET /posts?public=true            -> WHERE public = true
//	GET /posts?created_from=2024-01-01 -> WHERE created >= '2024-01-01'
//	GET /posts?created_to=2024-12-31   -> WHERE created <= '2024-12-31'
//
// Filters combine with AND, and with the built-in search/sort/pagination params.
func (p *Posts) filters() *Filedset {
	return NewFieldset(
		// Case-insensitive substring match on the title column.
		NewFilter("title", TYPE_STRING).
			WithLabel("Title").
			Contains(),

		// Exact boolean match on the public flag. Admin-only.
		NewFilter("public", TYPE_CHECKBOX).
			WithLabel("Public").
			AsAdminOnly().
			Equals(),

		// Admin-only: filter posts by their creator, searched by user name.
		NewFilter("user", TYPE_STRING).
			WithLabel("User").
			AsAdminOnly().
			Contains().
			WithSQLWhere("created_by IN (SELECT id FROM users WHERE user_name ILIKE %s)"),

		// Admin-only: filter posts by their creator's group (primary or additional),
		// searched by group name.
		NewFilter("user_group", TYPE_STRING).
			WithLabel("User Group").
			AsAdminOnly().
			Contains().
			WithSQLWhere("created_by IN ("+
				"SELECT uid FROM ("+
				"  SELECT id AS uid, user_group AS gid FROM users "+
				"  UNION ALL "+
				"  SELECT user_id AS uid, group_id AS gid FROM user_group_members"+
				") ug WHERE ug.gid IN (SELECT id FROM user_groups WHERE name ILIKE %s))"),

		// Date range against the created column. The filter parameter names
		// (created_from / created_to) differ from the column (created), which is
		// set with WithSQL; the ::timestamptz cast lets a plain date string bind
		// against the timestamp column.
		NewFilter("created_from", TYPE_DATE).
			WithLabel("Created from").
			WithSQL("created").
			WithSQLWhere("created >= %s::timestamptz"),

		NewFilter("created_to", TYPE_DATE).
			WithLabel("Created to").
			WithSQL("created").
			WithSQLWhere("created <= %s::timestamptz"),
	)
}
