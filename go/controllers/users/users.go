package users

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/ws117z5/tls-rest/go/lib/db/pgdb"

	"github.com/ws117z5/tls-rest/go/lib"

	"github.com/go-pg/urlstruct"
	"github.com/gorilla/mux"
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

// List all users
func List(w http.ResponseWriter, r *http.Request) {
	//var users []User
	//var values = pager.Values(r.URL.Query())

	users := make([]User, 0)
	filter := new(Filter)
	ctx := new(context.Context)
	keys := r.URL.Query()
	err := urlstruct.Unmarshal(*ctx, keys, filter)
	if err != nil {
		panic(err)
	}

	db, _ := pgdb.GetInstance()
	err = db.Model(&users).
		//TODO fix this
		//WhereStruct(filter).
		Limit(filter.Pager.GetLimit()).
		Offset(filter.Pager.GetOffset()).

		//Apply(pager.Pagination(values)).
		//Apply(orm.URLFilters(r.URL.Query())).
		Select()

	if err != nil {
		log.Panic(err)
	}

	err = json.NewEncoder(w).Encode(Data{lib.GetFields(User{}), users})
	if err != nil {
		log.Panic(err)
		fmt.Fprintln(w, "List Users!")
	}

}

// Create creates a user based on post request data
func Create(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	if name, ok := vars["vkid"]; ok {
		fmt.Fprintln(w, "Todo show:", name)
	} else {

	}
}

// GetInfo returns information about user
func GetInfo(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	if authType, ok := vars["authType"]; ok {
		if authType == "vk" {
			//resp, err := http.Get("http://example.com/")
			fmt.Println(authType)
		}
	} else {

	}
}

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
