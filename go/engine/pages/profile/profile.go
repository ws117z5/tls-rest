// Package profile is the current user's profile Page: a single, rights-filtered
// fieldset representation (no modes) backed by /api/profile. It is a PageAbstract
// declaration — the generic GET/PUT come from the engine; this package only says
// what the fields are and how to load/save the record (the current user's row).
package profile

import (
	"context"
	"errors"

	"tls-rest/go/app"
	appconfig "tls-rest/go/engine/controllers/config"
	"tls-rest/go/engine/controllers/db/cache"
	"tls-rest/go/engine/controllers/db/pgdb"
	"tls-rest/go/engine/controllers/field"
	"tls-rest/go/engine/controllers/functions"
	"tls-rest/go/engine/controllers/module"
)

// Page defines its own fields (excludes privilege/system columns) so a normal user can view/edit their own basic details.
var Page = &module.PageAbstract{
	ID:           "profile",
	Name:         "Profile",
	Icon:         "profile",
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
			WithLabel("Avatar URL").
			WithMode(field.MODE_LIST|field.MODE_VIEW|field.MODE_EDIT).
			WithOption("folderTemplate", "profile"),
		field.NewField("theme", field.TYPE_SELECT, false).
			WithLabel("Theme").
			WithMode(field.MODE_LIST | field.MODE_VIEW | field.MODE_EDIT).
			WithOptions(func() []map[string]interface{} {
				return []map[string]interface{}{
					{"value": "light", "name": "Light"},
					{"value": "dark", "name": "Dark"},
				}
			}),
	},

	Load: func(ctx context.Context, s *cache.Session) (map[string]interface{}, error) {

		db, err := pgdb.GetInstanceCtx(ctx)
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
		if cfg, err := db.GetOne("SELECT theme FROM config WHERE scope = 'user' AND scope_id = $1 LIMIT 1", s.UserID); err == nil && cfg != nil {
			row["theme"] = cfg["theme"]
		}
		return row, nil
	},

	Save: func(ctx context.Context, s *cache.Session, data map[string]interface{}) error {
		if len(data) == 0 {
			return nil
		}
		db, err := pgdb.GetInstanceCtx(ctx)
		if err != nil {
			return err
		}
		if theme, ok := data["theme"]; ok {
			delete(data, "theme")
			if err := saveUserTheme(db, s.UserID, theme); err != nil {
				return err
			}
		}
		if len(data) == 0 {
			return nil
		}
		_, err = db.UpdateRow("users", data, "id", s.UserID)
		return err
	},
}

// saveUserTheme creates or updates the caller's personal theme override (config: scope=user, scope_id=userID).
func saveUserTheme(db *pgdb.Db, userID int, theme interface{}) error {
	row, err := db.GetOne("SELECT id FROM config WHERE scope = 'user' AND scope_id = $1 LIMIT 1", userID)
	if err != nil {
		return err
	}
	if row != nil {
		_, err = db.UpdateRow("config", map[string]interface{}{"theme": theme}, "id", functions.Coerce[int](row["id"]))
	} else {
		_, err = db.InsertRow("config", map[string]interface{}{"scope": "user", "scope_id": userID, "theme": theme})
	}
	if err == nil {
		appconfig.BumpConfigEpoch()
	}
	return err
}

func init() {
	app.RegisterPage(Page)
}
