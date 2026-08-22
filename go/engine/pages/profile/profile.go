// Package profile is the current user's profile Page: a single, rights-filtered
// fieldset representation (no modes) backed by /api/profile. It is a PageAbstract
// declaration — the generic GET/PUT come from the engine; this package only says
// what the fields are and how to load/save the record (the current user's row).
package profile

import (
	"errors"
	"tls-rest/go/engine/controllers/db/cache"
	"tls-rest/go/engine/controllers/db/pgdb"
	"tls-rest/go/engine/controllers/field"
	"tls-rest/go/engine/controllers/module"
)

// Page defines its own fields, independent of the admin-only users module, so a
// normal user can view/edit their own basic details. Privilege fields
// (user_group / access / system columns) are deliberately excluded.
var Page = &module.PageAbstract{
	ID:           "profile",
	Name:         "Profile",
	Endpoint:     "/api/profile",
	Editable:     true,
	RequiresAuth: true,
	Fields: []field.Field{
		field.NewField("user_name", field.TYPE_STRING, true).
			WithLabel("Username").WithMode(field.MODE_LIST | field.MODE_VIEW | field.MODE_EDIT),
		field.NewField("first_name", field.TYPE_STRING, false).
			WithLabel("First name").WithMode(field.MODE_LIST | field.MODE_VIEW | field.MODE_EDIT),
		field.NewField("last_name", field.TYPE_STRING, false).
			WithLabel("Last name").WithMode(field.MODE_LIST | field.MODE_VIEW | field.MODE_EDIT),
		field.NewField("email", field.TYPE_STRING, true).
			WithLabel("Email").WithMode(field.MODE_LIST | field.MODE_VIEW | field.MODE_EDIT),
		field.NewField("image", field.TYPE_IMAGE, false).
			WithLabel("Avatar URL").WithMode(field.MODE_LIST | field.MODE_VIEW | field.MODE_EDIT),
	},

	Load: func(s *cache.Session) (map[string]interface{}, error) {

		db, err := pgdb.GetInstance()
		if err != nil {
			return nil, err
		}
		row, err := db.GetOne(
			`SELECT id, user_name, first_name, last_name, email, image FROM users WHERE id = $1`,
			s.UserID,
		)
		if err != nil {
			return nil, err
		}
		if len(row) == 0 {
			return nil, errors.New("No enrty found")
		}
		return row, nil
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

func Init() {
	Page.Initialize()
}
