package users

import (
	"time"

	"github.com/go-pg/urlstruct"

	. "tls-rest/go/engine"
)

// UserObj represents the data structure for users
type UserObj struct {
	ID          int64     `json:"id" db:"id"`
	UUID        string    `json:"uuid" db:"uuid"`
	UserName    string    `json:"user_name" db:"user_name"`
	FirstName   string    `json:"first_name" db:"first_name"`
	LastName    string    `json:"last_name" db:"last_name"`
	Email       string    `json:"email" db:"email"`
	PhoneNumber string    `json:"phone_number" db:"phone_number"`
	Created     time.Time `json:"created" db:"created"`
	Updated     time.Time `json:"updated" db:"updated"`
	CreatedBy   int       `json:"created_by" db:"created_by"`
}

// TableName returns the database table name
func (UserObj) TableName() string {
	return "users"
}

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
			ID:     "users",
			Name:   "Users Management",
			Rights: make(map[int]int),
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
