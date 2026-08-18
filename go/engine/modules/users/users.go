package users

import (
	"tls-rest/go/lib/db/pgdb"

	. "tls-rest/go/engine"
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
			WithLabel("User Group").
			WithDescription("Group this user belongs to").
			WithOption("widget", "select").
			WithOption("dataSource", "user_groups").
			WithOption("valueField", "id").
			WithOption("displayField", "name"),
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

	if rows, e := db.RQuery(
		`SELECT id, user_name FROM users WHERE lower(email) = lower($1) LIMIT 1`, s.Email,
	); e == nil && len(rows) > 0 {
		return pgdb.AsInt64(rows[0]["id"]), pgdb.AsString(rows[0]["user_name"]), nil
	}

	lastInitial := ""
	if len(s.LastName) > 0 {
		lastInitial = " " + s.LastName[0:1] + "."
	}
	username := s.FirstName + lastInitial

	ins, err := db.RQuery(
		`INSERT INTO users (user_name, first_name, last_name, email, image)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		username, s.FirstName, s.LastName, s.Email, s.Image,
	)
	if err != nil || len(ins) == 0 {
		return 0, "", err
	}
	return pgdb.AsInt64(ins[0]["id"]), username, nil
}

func init() {
	// Create module instance
	UserModule = NewUsers()
	// Initialize with database table - routes are automatically registered
	UserModule.Initialize("users")
}
