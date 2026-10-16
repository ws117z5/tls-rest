package users

import (
	"context"

	"tls-rest/go/app"
	"tls-rest/go/engine/controllers/db/pgdb"
	"tls-rest/go/engine/controllers/functions"
	"tls-rest/go/engine/controllers/module"

	. "tls-rest/go/engine/controllers/field"
)

// assignableGroups lists the groups the requesting user may grant, calculated
// from their own authority (passed in by the engine): an admin may assign any
// group; anyone else only groups at or below their access level, and never an
// admin group. Change this closure to change the policy — the engine has no
// knowledge of it.
func assignableGroups(ctx context.Context, viewer map[string]interface{}) []map[string]interface{} {
	db, err := pgdb.GetInstanceCtx(ctx)
	if err != nil {
		return nil
	}
	isAdmin, _ := viewer["isAdmin"].(bool)
	level := functions.Int(viewer["level"])
	rows, err := db.GetAll(
		`SELECT id AS value, name FROM user_groups
		 WHERE $1 OR (id <= $2 AND NOT is_admin)
		 ORDER BY name`,
		isAdmin, level)
	if err != nil {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]interface{}{
			"value": r["value"],
			"name":  functions.Coerce[string](r["name"]),
		})
	}
	return out
}

// roleLabel renders a stored role value for display — the same labels
// modulerights.go's roleOptions() uses.
func roleLabel(role string) string {
	switch role {
	case "moderator":
		return "Moderator"
	case "group_admin":
		return "Group Admin"
	default:
		return "User"
	}
}

// roleOptionsForUser lists the roles assignable to the record being edited:
// "user" (the no-op default) always, plus any other role that has at least
// one user_role_rights ceiling defined for one of THIS user's own groups —
// you can't be made a moderator/group_admin until someone has actually
// defined what that means for your group. Reads the edited record's own id
// (values["id"]) rather than the viewer's authority, which is why this is a
// WithAutocomplete function (gets record values) and not WithOptionsCtx
// (gets only the viewer's).
func roleOptionsForUser(ctx context.Context, _ string, values map[string]interface{}) []AutoOption {
	always := []AutoOption{{Value: "user", Label: roleLabel("user")}}
	id := functions.Int(values["id"])
	if id <= 0 {
		return always
	}
	db, err := pgdb.GetInstanceCtx(ctx)
	if err != nil {
		return always
	}
	rows, err := db.GetAll(`
		SELECT DISTINCT role FROM user_role_rights
		WHERE group_id IN (SELECT jsonb_array_elements_text(groups)::int FROM users WHERE id = $1)
	`, id)
	if err != nil {
		return always
	}
	out := always
	for _, r := range rows {
		if role := functions.Coerce[string](r["role"]); role != "" && role != "user" {
			out = append(out, AutoOption{Value: role, Label: roleLabel(role)})
		}
	}
	return out
}

// fieldset defines the module's fields (default system fields are added
// automatically by Initialize()).
func (u *Users) fieldset() []Field {
	return []Field{
		NewField("user_name", TYPE_STRING, false).
			WithLabel("Username").
			WithDescription("User's display name").
			WithValidation("minLength", 2).
			WithValidation("maxLength", 100),

		NewField("first_name", TYPE_STRING, true).
			WithLabel("First Name").
			WithDescription("User's first name").
			WithValidation("minLength", 2).
			WithValidation("maxLength", 50),

		NewField("last_name", TYPE_STRING, false).
			WithLabel("Last Name").
			WithDescription("User's last name").
			WithValidation("maxLength", 60),

		NewField("email", TYPE_STRING, true).
			WithLabel("Email Address").
			WithDescription("User's email address").
			WithValidation("email", true).
			WithValidation("unique", true),

		NewField("image", TYPE_IMAGE, false).
			WithLabel("Profile Image").
			WithDescription("User's profile image").
			WithOption("folderTemplate", "profile").
			NonSearchable().
			WithMode(MODE_VIEW | MODE_EDIT).
			WithResize(ResizeOptions{Height: 300}),

		// Role within the user's own group(s) — user/moderator/group_admin. A role
		// only narrows what the group already grants (auth.resolveSessionRights
		// applies it as a ceiling over group rights, never over individually
		// granted user_rights); it can never elevate a user above their group.
		// Only offers moderator/group_admin once someone has actually defined a
		// user_role_rights ceiling for this user's own group — see
		// roleOptionsForUser. See modulerights.go's RoleRightsModule.
		NewField("user_role", TYPE_SELECT, false).
			WithLabel("Role").
			WithDescription("Privileges within the user's own group — never above it").
			WithDefault("user").
			WithOption("widget", "select").
			WithAutocomplete(map[string]interface{}{"function": roleOptionsForUser}),

		// Group membership: a jsonb array of user_groups ids stored in the real
		// users.groups column (created by the engine because this TYPE_TABLE
		// field has a submit hook). The highest id is the user's access level;
		// membership of an is_admin group grants admin.
		//
		// It is deliberately NOT in any SELECT — both the view and the edit form
		// load the current rows from the TableData endpoint, so an untouched save
		// leaves the column alone instead of overwriting it with a projection.
		// The select is authority-scoped (assignableGroups): a non-admin editor
		// only sees groups at or below their own level, never an admin group.
		NewField("groups", TYPE_TABLE, false).
			WithLabel("Groups").
			WithDescription("Groups this user belongs to (highest id = access level; an admin group grants admin)").
			InModes(MODE_VIEW | MODE_EDIT | MODE_SUBMIT). // not in any SELECT, so a list column would only ever show "—"
			TableFieldset([]Field{
				NewField("group", TYPE_INT, true).
					WithLabel("Group").
					WithOption("widget", "select").
					WithOptionsCtx(assignableGroups),
			}).
			TableRowsAddable("group").
			TableData(func(ctx context.Context, data map[string]interface{}) []map[string]interface{} {
				uid := functions.Int(data["id"])
				if uid <= 0 {
					return nil
				}
				db, err := pgdb.GetInstanceCtx(ctx)
				if err != nil {
					return nil
				}
				rows, err := db.GetAll(
					`SELECT jsonb_array_elements_text(groups)::int AS "group" FROM users WHERE id = $1`,
					uid)
				if err != nil {
					return nil
				}
				return rows
			}).
			TableOnSubmit(func(rows []map[string]interface{}) interface{} {
				ids := []interface{}{}
				for _, r := range rows {
					if id := functions.Int(r["group"]); id != -1 {
						ids = append(ids, id)
					}
				}
				return ids
			}),
	}
}

// Global module instance (initialized at startup)
var UserModule *Users

// FindOrCreateGoogleUser looks up a user by email (case-insensitive) and creates
// one if absent, returning the user's id and display name. Used by the OAuth
// callback to establish a session. Idempotent across repeated logins.
func FindOrCreateGoogleUser(s *GoogleAccount) (int64, string, error) {
	db, err := pgdb.GetInstance()
	if err != nil {
		return 0, "", err
	}

	if row, e := db.GetOne(
		`SELECT id, user_name FROM users WHERE lower(email) = lower($1) LIMIT 1`, s.Email,
	); e == nil && row != nil {
		return functions.Coerce[int64](row["id"]), functions.Coerce[string](row["user_name"]), nil
	}

	lastInitial := ""
	if len(s.LastName) > 0 {
		lastInitial = " " + s.LastName[0:1] + "."
	}
	username := s.FirstName + lastInitial

	id, err := db.InsertRow("users", map[string]interface{}{
		"user_name":  username,
		"first_name": s.FirstName,
		"last_name":  s.LastName,
		"email":      s.Email,
		"image":      s.Image,
	})
	if err != nil {
		return 0, "", err
	}
	return id, username, nil
}

// NewUsers creates a new Users module instance
func NewUsers() *Users {
	module := &Users{
		ModuleAbstract: &module.ModuleAbstract[interface{}]{
			ID:              "users",
			RightsAffecting: true,
			Name:            "Users",
			Icon:            "users",
			Submenu:         "users",
			Rights:          make(map[int]int),
			// Administration module: no access unless explicitly granted (or admin).
			DefaultPermission:    0, // PERMISSION_DENY
			DefaultPermissionSet: true,
		},
	}

	// Build the field set. Default system fields are added automatically by
	// Initialize().
	module.Fields = module.fieldset()

	return module
}

func init() {
	UserModule = NewUsers()
	app.RegisterModule(UserModule, "users")
}

// All CRUD operations are handled automatically by the module system.
// Routes are automatically registered when Initialize() is called.
