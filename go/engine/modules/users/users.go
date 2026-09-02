package users

import (
	"tls-rest/go/engine/controllers/db/pgdb"
	"tls-rest/go/engine/controllers/functions"
	"tls-rest/go/engine/controllers/module"

	. "tls-rest/go/engine/controllers/field"
)

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

		// Single group membership, stored as an integer FK on the user
		// (users.user_group -> user_groups.id). The group id is the user's
		// access level; the group's is_admin flag confers admin. Rendered as a
		// select; stored as an integer.
		NewField("user_group", TYPE_INT, false).
			WithLabel("Primary Group").
			WithDescription("The user's primary group (access level). Additional groups are managed under Group Members and shown below.").
			WithOption("widget", "select").
			WithOption("dataSource", "user_groups").
			WithOption("valueField", "id").
			WithOption("displayField", "name"),

		// All groups this user belongs to (primary + additional memberships),
		// shown as a table. Read-only here; edit memberships via the Group Members
		// module. Rows are provided server-side from the id posted in ctx.
		NewField("groups", TYPE_TABLE, false).
			WithLabel("Groups").
			WithDescription("Every group this user belongs to").
			AsVirtual().
			AsReadOnly().
			TableFieldset([]Field{
				NewField("group", TYPE_STRING, false).WithLabel("Group").AsReadOnly(),
			}).
			TableData(func(ctx map[string]interface{}) []map[string]interface{} {
				uid := functions.Int(ctx["id"])
				if uid <= 0 {
					return nil
				}
				db, err := pgdb.GetInstance()
				if err != nil {
					return nil
				}
				rows, err := db.RQuery(`
					SELECT ug.name AS group
					FROM user_groups ug
					WHERE ug.id IN (
						SELECT user_group FROM users WHERE id = $1 AND user_group IS NOT NULL
						UNION
						SELECT group_id FROM user_group_members WHERE user_id = $1
					)
					ORDER BY ug.name
				`, uid)
				if err != nil {
					return nil
				}
				return rows
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
