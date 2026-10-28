package words

import (
	. "tls-rest/go/engine/controllers/field"
)

// filters declares the list-mode filters for GET /words. Owner scoping (only
// the caller's own rows, unless admin) is enforced by the engine via the
// module's OwnerScoped flag — an admin sees every user's words by default,
// narrowed to their own with ?own=true (see fieldset_engine.go's
// buildScopeConditions) — independently of these filters. "user" below is
// the admin-only complement: viewing one specific OTHER user's words.
func (m *Words) filters() *Filedset {
	return NewFieldset(
		NewFilter("word", TYPE_STRING).
			WithLabel("Word").
			Contains(),
		NewFilter("created_by", TYPE_INT).
			WithLabel("User").
			Equals().
			AsAdminOnly(),
	)
}
