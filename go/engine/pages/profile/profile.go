// Package profile is the current user's profile Page: a single, rights-filtered
// fieldset representation (no modes) backed by /api/profile. It is a PageAbstract
// declaration — the generic GET/PUT come from the engine; this package only says
// what the fields are and how to load/save the record (the current user's row).
package profile

import (
	"context"
	"errors"
	"strings"

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
	Routes: []module.PageRoute{
		{Path: "/api/profile/google/connect", Methods: []string{"GET"}, Handler: handleGoogleConnect},
		{Path: "/api/profile/google/callback", Methods: []string{"GET"}, Handler: handleGoogleCallback},
		{Path: "/api/profile/google/disconnect", Methods: []string{"POST"}, Handler: handleGoogleDisconnect},
		{Path: "/api/profile/google/status", Methods: []string{"GET"}, Handler: handleGoogleStatus},
		{Path: "/api/profile/claude-token", Methods: []string{"POST"}, Handler: handleGenerateClaudeToken},
	},
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
		// Read-only: lets a client show "what am I" (e.g. an account screen's
		// rights info) without a second request — handlePut already drops any
		// field with ReadOnly set, so this can never be used to self-elevate.
		field.NewField("user_role", field.TYPE_STRING, false).
			WithLabel("Role").
			WithMode(field.MODE_LIST | field.MODE_VIEW).
			AsReadOnly(),
		// Gates /api/users/search's name-matching branch (go/engine/modules/chats/search.go) — off by
		// default, since a username is already how people find you deliberately.
		field.NewField("allow_search_by_name", field.TYPE_CHECKBOX, false).
			WithLabel("Let others find me by first/last name").
			WithMode(field.MODE_LIST | field.MODE_VIEW | field.MODE_EDIT),
		// Gates whether a non-friend can add this user to a group chat at all
		// (go/engine/modules/chats' group-add gating) — a friend always can.
		field.NewField("allow_chats_from_non_friends", field.TYPE_CHECKBOX, false).
			WithLabel("Let non-friends add me to group chats").
			WithMode(field.MODE_LIST | field.MODE_VIEW | field.MODE_EDIT),
		field.NewField("theme", field.TYPE_SELECT, false).
			WithLabel("Theme").
			WithMode(field.MODE_LIST | field.MODE_VIEW | field.MODE_EDIT).
			WithOptions(func() []map[string]interface{} {
				return []map[string]interface{}{
					{"value": "light", "name": "Light"},
					{"value": "dark", "name": "Dark"},
				}
			}),
		// Default state of a new event's own "sync with Google Calendar"
		// checkbox (events.go's sync_with_google) — per-user, same config-row
		// override mechanism as theme. Only meaningful once Google Calendar is
		// connected below, but left always editable like theme is.
		field.NewField("sync_with_google_default", field.TYPE_CHECKBOX, false).
			WithLabel("New events sync with Google Calendar by default").
			WithMode(field.MODE_LIST | field.MODE_VIEW | field.MODE_EDIT),
	},

	Load: func(ctx context.Context, s *cache.Session) (map[string]interface{}, error) {

		db, err := pgdb.GetInstanceCtx(ctx)
		if err != nil {
			return nil, err
		}
		row, err := db.GetOne(
			`SELECT id, user_name, first_name, last_name, email, image, user_role,
			        allow_search_by_name, allow_chats_from_non_friends
			 FROM users WHERE id = $1`,
			s.UserID,
		)
		if err != nil {
			return nil, err
		}
		if len(row) == 0 {
			return nil, errors.New("No enrty found")
		}
		if cfg, err := db.GetOne("SELECT theme, sync_with_google_default FROM config WHERE scope = 'user' AND scope_id = $1 LIMIT 1", s.UserID); err == nil && cfg != nil {
			row["theme"] = cfg["theme"]
			if v := cfg["sync_with_google_default"]; v != nil {
				row["sync_with_google_default"] = functions.Coerce[string](v) != "false"
			} else {
				row["sync_with_google_default"] = true // unset = the field's own default
			}
		} else {
			row["sync_with_google_default"] = true
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
			if err := saveUserConfigValue(db, s.UserID, "theme", theme); err != nil {
				return err
			}
		}
		if sync, ok := data["sync_with_google_default"]; ok {
			delete(data, "sync_with_google_default")
			value := "true"
			if !functions.Truthy(sync) {
				value = "false"
			}
			if err := saveUserConfigValue(db, s.UserID, "sync_with_google_default", value); err != nil {
				return err
			}
		}
		if len(data) == 0 {
			return nil
		}
		if email, ok := data["email"].(string); ok {
			email = strings.TrimSpace(strings.ToLower(email))
			data["email"] = email
			if taken, err := db.GetOne("SELECT id FROM users WHERE lower(email) = $1 AND id <> $2 LIMIT 1", email, s.UserID); err != nil {
				return err
			} else if taken != nil {
				return errors.New("this email is already in use")
			}
		}
		_, err = db.UpdateRow("users", data, "id", s.UserID)
		return err
	},
}

// saveUserConfigValue creates or updates one column of the caller's personal
// config override row (scope=user, scope_id=userID) — theme and
// sync_with_google_default both go through this.
func saveUserConfigValue(db *pgdb.Db, userID int, column string, value interface{}) error {
	row, err := db.GetOne("SELECT id FROM config WHERE scope = 'user' AND scope_id = $1 LIMIT 1", userID)
	if err != nil {
		return err
	}
	if row != nil {
		_, err = db.UpdateRow("config", map[string]interface{}{column: value}, "id", functions.Coerce[int](row["id"]))
	} else {
		_, err = db.InsertRow("config", map[string]interface{}{"scope": "user", "scope_id": userID, column: value})
	}
	if err == nil {
		appconfig.BumpConfigEpoch()
	}
	return err
}

func init() {
	app.RegisterPage(Page)
}
