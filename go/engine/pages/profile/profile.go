// Package profile is the current user's profile Page: a single, rights-filtered
// fieldset representation (no modes) backed by /api/profile. It is a PageAbstract
// declaration — the generic GET/PUT come from the engine; this package only says
// what the fields are and how to load/save the record (the current user's row).
package profile

import (
	engine "tls-rest/go/engine"
	"tls-rest/go/lib/db/cache"
	pgdb "tls-rest/go/lib/db/pgdb"
)

// Page defines its own fields, independent of the admin-only users module, so a
// normal user can view/edit their own basic details. Privilege fields
// (user_group / access / system columns) are deliberately excluded.
var Page = &engine.PageAbstract{
	ID:           "profile",
	Name:         "Profile",
	Endpoint:     "/api/profile",
	Editable:     true,
	RequiresAuth: true,
	Fields: []engine.Field{
		engine.NewField("user_name", engine.TYPE_STRING, true).
			WithLabel("Username").WithMode(engine.MODE_LIST | engine.MODE_VIEW | engine.MODE_EDIT),
		engine.NewField("first_name", engine.TYPE_STRING, false).
			WithLabel("First name").WithMode(engine.MODE_LIST | engine.MODE_VIEW | engine.MODE_EDIT),
		engine.NewField("last_name", engine.TYPE_STRING, false).
			WithLabel("Last name").WithMode(engine.MODE_LIST | engine.MODE_VIEW | engine.MODE_EDIT),
		engine.NewField("email", engine.TYPE_STRING, true).
			WithLabel("Email").WithMode(engine.MODE_LIST | engine.MODE_VIEW | engine.MODE_EDIT),
		engine.NewField("image", engine.TYPE_STRING, false).
			WithLabel("Avatar URL").WithMode(engine.MODE_LIST | engine.MODE_VIEW | engine.MODE_EDIT),
	},

	Load: func(s *cache.Session) (map[string]interface{}, error) {
		db, err := pgdb.GetInstance()
		if err != nil {
			return nil, err
		}
		rows, err := db.RQuery(
			`SELECT id, user_name, first_name, last_name, email, image FROM users WHERE id = $1`,
			s.UserID,
		)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return map[string]interface{}{}, nil
		}
		return rows[0], nil
	},

	Save: func(s *cache.Session, data map[string]interface{}) error {
		if len(data) == 0 {
			return nil
		}
		db, err := pgdb.GetInstance()
		if err != nil {
			return err
		}
		_, err = db.UpdateRow("users", data, "id", s.UserID)
		return err
	},
}

func init() {
	Page.Initialize()
}
