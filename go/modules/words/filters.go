package words

import (
	. "tls-rest/go/engine/controllers/field"
)

// filters declares the list-mode filters for GET /words. Owner scoping (only the
// caller's own rows) is enforced by the engine via the module's OwnerScoped flag,
// independently of these filters.
func (m *Words) filters() *Filedset {
	return NewFieldset(
		NewFilter("word", TYPE_STRING).
			WithLabel("Word").
			Contains(),
	)
}
