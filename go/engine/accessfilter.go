package module

import (
	"net/http"

	"tls-rest/go/lib/db/cache"
	"tls-rest/go/lib/db/pgdb"
)

// This file centralises request-time access filtering for the fieldset engine.
// Two distinct rules apply:
//
//   * schema/display visibility (which columns a user is shown): system fields
//     are admin-only, and any field whose required Access level exceeds the
//     user's level is hidden.
//
//   * data readability (which column values leave the server): system fields
//     are always kept (the SPA needs id/uuid to route and act), but genuinely
//     restricted *non-system* fields (Access > level) are stripped from the
//     query projection so their values are never returned.
//
// Row visibility (which records a user may see at all) is handled separately by
// the access WHERE clause in the fieldset engine.

// systemFields are engine-managed fields: shown only to admins, and (except
// access) never editable. Kept in returned data regardless, because the client
// needs id/uuid to identify and open records.
var systemFields = map[string]bool{
	"id":         true,
	"uuid":       true,
	"created":    true,
	"updated":    true,
	"created_by": true,
	"created_at": true,
	"updated_at": true,
	"access":     true,
}

// IsSystemField reports whether a field is engine-managed (admin-only display).
func IsSystemField(name string) bool {
	return systemFields[name]
}

// adminOnlyData lists system fields whose *values* are withheld from non-admins
// entirely (not just hidden from the schema). uuid qualifies: a non-admin never
// receives it. id does NOT — the client routes and acts on records by id, so it
// must remain readable even when the uuid is not.
var adminOnlyData = map[string]bool{
	"uuid": true,
}

// viewer is the access context of the current request.
type viewer struct {
	isAdmin bool
	level   int
}

// viewerFromRequest derives the viewer from the session on the request context.
// A missing session (nil request or unauthenticated internal call) is treated
// as the most restrictive anonymous viewer.
func viewerFromRequest(r *http.Request) viewer {
	if r == nil {
		return viewer{}
	}
	s := cache.SessionFromContext(r.Context())
	if s == nil {
		return viewer{}
	}
	return viewer{isAdmin: s.IsAdmin, level: s.AccessLevel}
}

// CanViewRecord reports whether the request's viewer may read a specific record
// of a table, applying the same row-level access rule the fieldset engine uses
// for lists: admins see everything; everyone else sees a row only when its
// access level is within their own. This is the single place that answers
// "can this user access this record" — features (e.g. image serving) call it
// instead of re-implementing the check.
func CanViewRecord(r *http.Request, tableName string, id int64) (bool, error) {
	v := viewerFromRequest(r)

	db, err := pgdb.GetInstance()
	if err != nil {
		return false, err
	}
	row, err := db.GetOne("SELECT access FROM "+db.Quote(tableName)+" WHERE id = $1", id)
	if err != nil {
		return false, err
	}
	if row == nil {
		return false, nil
	}
	if v.isAdmin {
		return true, nil
	}
	access := 0
	if row["access"] != nil {
		access = pgdb.Coerce[int](row["access"])
	}
	return access <= v.level, nil
}

// fieldVisibleInSchema reports whether a field should be described to the user
// (i.e. appear as a column/form field). System fields are admin-only; access-
// gated fields are hidden from lower-level users.
func (v viewer) fieldVisibleInSchema(f Field) bool {
	if v.isAdmin {
		return true
	}
	if IsSystemField(f.Name) {
		return false
	}
	if f.Access > v.level {
		return false
	}
	return true
}

// fieldReadableInData reports whether a field's value may be returned in data.
// System fields are generally readable (the client needs them), except those in
// adminOnlyData (e.g. uuid) which are withheld from non-admins entirely; other
// non-system access-gated fields are withheld from users below the required level.
func (v viewer) fieldReadableInData(f Field) bool {
	if v.isAdmin {
		return true
	}
	if adminOnlyData[f.Name] {
		return false
	}
	if IsSystemField(f.Name) {
		return true
	}
	return f.Access <= v.level
}
