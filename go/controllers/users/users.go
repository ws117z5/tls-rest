package users

import (
	"time"

	"github.com/ws117z5/tls-rest/go/lib/db/pgdb"
	. "github.com/ws117z5/tls-rest/go/lib/module"

	"github.com/go-pg/urlstruct"
)

/*
https://localhost/users/Auth/GoogleCallback?
state=f%2BBGgkH5XpAbJi%2FjDVjbLQ%3D%3D&code=4%2F0AVG7fiRfTIr9U2t
lOm3kX2Dg2KKkIrCQww8ArTESu6jLOat3hE2E_EE9ttUkDJgTRFmAQ&scope=em
ail+profile+https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fuserinfo.email+
openid+https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fuserinfo.profile&a
uthuser=0&prompt=none
*/

// GUser is a retrieved and authentiacted user.
type GUser struct {
	Sub           string `json:"sub"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Profile       string `json:"profile"`
	Picture       string `json:"picture"`
	Email         string `json:"email"`
	EmailVerified string `json:"email_verified"`
	Gender        string `json:"gender"`
}

// GoogleTokenResp token aquired from google oauth
type GoogleTokenResp struct {
	Expires      int64  `json:"expires_in"`
	Token        string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
	TokenType    string `json:"token_type"`
	TokenID      string `json:"id_token"`
}

// GoogleAccount google auth response
type GoogleAccount struct {
	ID            string `json:"id"` //`json:"id,string"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"verified_email"`
	FullName      string `json:"name"`
	FirstName     string `json:"given_name"`
	LastName      string `json:"family_name"`
	Image         string `json:"picture"`
	Locale        string `json:"locale"`
}

// GoogleSessions google token
type GoogleSessions struct {
	UUID      string `json:"access_token"`
	TokenInfo int64  `json:"token_info"`
}

// User main description
type User struct {
	ID        int64     `json:"id" pg:"id"`
	Uuid      string    `pg:"uuid, default:uuid_generate_v4()"`
	Username  string    `json:"user_name" pg:"user_name"`
	Firstname string    `json:"first_name" pg:"first_name"`
	Lastname  string    `json:"last_name" pg:"last_name"`
	Email     string    `json:"-"`
	Image     string    `json:"image" pg:"image"`
	CreatedAt time.Time `pg:"created_at,default:now()" json:"created_at"`
	UpdatedAt time.Time `pg:"updated_at,default:now()" json:"updated_at"`
}

type UserExtended struct {
	*User
	Image     string   `json:"image"`
	tableName struct{} `pg:"users"`
}

// UserDb user extended with db
type UserDb struct {
	User
	tableName struct{} `sql:"users,alias:users"`

	//Books []Book `pg:"many2many:book_genres"` // many to many relation

	//ParentId  int
	//Subgenres []Genre `pg:"fk:parent_id"` // fk specifies foreign key
}

// Credentials who knows
type Credentials struct {
	Cid     string `json:"cid"`
	Csecret string `json:"csecret"`
}

type Filter struct {
	tableName struct{} `urlstruct:"b"`

	urlstruct.Pager
}

type Data struct {
	Fieldset map[string]string
	Data     []User
}

// Users module following CInvoice paradigm
type Users struct {
	*ModuleAbstract[interface{}]
}

// NewUsers creates a new Users module instance
func NewUsers() *Users {
	module := &Users{
		ModuleAbstract: &ModuleAbstract[interface{}]{
			ID:                "users",
			Name:              "Users Management",
			Rights:            make(map[int]int),
			DefaultPermission: 2, // PERMISSION_WRITE - Users module requires write access by default
		},
	}

	// Initialize fields in dedicated method (like PHP init() method)
	module.init()

	return module
}

// init method defines all fields and module configuration (like CInvoice.php init method)
func (u *Users) init() {
	// Define module-specific fields (default fields auto-added by system)
	fields := []Field{
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

		NewField("image", TYPE_STRING, false).
			WithLabel("Profile Image").
			WithDescription("URL to user's profile image").
			NonSearchable().
			WithMode(MODE_VIEW | MODE_EDIT),

		NewField("user_groups", TYPE_TABLE, false).
			WithLabel("User Groups").
			WithDescription("Groups this user belongs to").
			WithTableQuery(`
				SELECT ug.name as group_name, ugm.role, ugm.created as joined_at
				FROM user_group_members ugm
				JOIN user_groups ug ON ug.id = ugm.group_id
				WHERE ugm.user_id = ?
			`).
			WithTableColumns([]string{"group_name", "role", "joined_at"}).
			WithTableEditable(true).
			WithTableSubmitFunction("updateUserGroups").
			WithTableRowActions([]map[string]interface{}{
				{"label": "Remove", "action": "removeFromGroup", "icon": "trash"},
				{"label": "Change Role", "action": "changeRole", "icon": "edit"},
			}),

		NewField("permissions", TYPE_TABLE, false).
			WithLabel("Direct Permissions").
			WithDescription("Direct permissions assigned to this user").
			WithTableQuery(`
				SELECT m.name as module_name, ur.rights, ur.created as granted_at
				FROM user_rights ur
				JOIN modules m ON m.id = ur.module_id
				WHERE ur.user_id = ?
			`).
			WithTableColumns([]string{"module_name", "rights", "granted_at"}).
			WithTableEditable(true).
			WithTableSubmitFunction("updateUserPermissions"),
	}

	u.Fields = fields
}

// Global module instance (initialized at startup)
var UserModule *Users

// RegisterGoogleUser Registers a user based on the reponse token from google oauth
func RegisterGoogleUser(s *GoogleAccount) {
	db, _ := pgdb.GetInstance()

	//if err != nil {
	_, err := db.Model(&User{
		Username:  (*s).FirstName + " " + string((*s).LastName[0]) + ".",
		Firstname: s.FirstName,
		Lastname:  s.LastName,
		Email:     s.Email,
		Image:     s.Image,
	}).Insert()

	if err != nil {
		panic(err)
	}
}

func init() {
	// Create module instance
	UserModule = NewUsers()

	// Initialize with database table - routes are automatically registered
	UserModule.Initialize("users")
}

// All CRUD operations are handled automatically by the module system.
// Routes are automatically registered when Initialize() is called.
