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

		// Exact boolean match on the public flag.
		NewFilter("public", TYPE_CHECKBOX).
			WithLabel("Public").
			Equals(),

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
