package users

import (
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
func assignableGroups(ctx map[string]interface{}) []map[string]interface{} {
	db, err := pgdb.GetInstance()
	if err != nil {
		return nil
	}
	isAdmin, _ := ctx["isAdmin"].(bool)
	level := functions.Int(ctx["level"])
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
			NonSearchable().
			WithMode(MODE_VIEW | MODE_EDIT),

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
			InModes(MODE_VIEW | MODE_EDIT). // not in any SELECT, so a list column would only ever show "—"
			TableFieldset([]Field{
				NewField("group", TYPE_INT, true).
					WithLabel("Group").
					WithOption("widget", "select").
					WithOptionsCtx(assignableGroups),
			}).
			TableRowsAddable("group").
			TableData(func(ctx map[string]interface{}) []map[string]interface{} {
				uid := functions.Int(ctx["id"])
				if uid <= 0 {
					return nil
				}
				db, err := pgdb.GetInstance()
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
			Submenu:         "engine",
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

func Init() {
	// Create module instance
	UserModule = NewUsers()

	// Initialize with database table - routes are automatically registered
	UserModule.Initialize("users")
}

// All CRUD operations are handled automatically by the module system.
// Routes are automatically registered when Initialize() is called.
