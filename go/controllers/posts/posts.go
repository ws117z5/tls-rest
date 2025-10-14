package posts

import (
	"net/http"
	"time"

	. "github.com/ws117z5/tls-rest/go/lib/module"
)

type Post struct {
	tableName struct{} `pg:"posts"`

	ID        int64     `json:"id"`
	UUID      string    `sql:"default:uuid_generate_v4()" json:"uuid"`
	Title     string    `json:"title"`
	Images    []string  `json:"images"`
	Content   string    `json:"content"`
	Created   time.Time `sql:"default:now()" json:"created"`
	CreatedBy string    `json:"created_by"`
}

// Initialize the module with fieldset configuration
var Module = &ModuleAbstract[interface{}]{
	ID:   "posts",
	Name: "Posts",
	Fields: []Field{
		NewField("id", TYPE_INT, false).
			WithLabel("ID").
			NonFilterable().
			AsReadOnly().
			WithMode(MODE_LIST | MODE_VIEW),

		NewField("uuid", TYPE_STRING, false).
			WithLabel("UUID").
			WithSQL("uuid_generate_v4()").
			AsReadOnly().
			NonFilterable().
			WithMode(MODE_LIST | MODE_VIEW),

		NewField("title", TYPE_STRING, true).
			WithLabel("Title").
			WithDescription("Post title").
			WithValidation("minLength", 3).
			WithValidation("maxLength", 200),

		NewField("images", TYPE_JSON, false).
			WithLabel("Images").
			WithDescription("Post images array").
			NonSortable().
			NonSearchable(),

		NewField("content", TYPE_TEXT, true).
			WithLabel("Content").
			WithDescription("Post content").
			WithValidation("minLength", 10).
			NonSortable(),

		NewField("created", TYPE_DATE_TIME, false).
			WithLabel("Created").
			WithSQL("now()").
			AsReadOnly().
			NonFilterable().
			WithMode(MODE_LIST | MODE_VIEW),

		NewField("created_by", TYPE_STRING, false).
			WithLabel("Created By").
			AsReadOnly().
			NonFilterable().
			WithMode(MODE_LIST | MODE_VIEW),
	},
	Rights: make(map[int]int),
}

// Create controller instance
var controller = NewBaseController(Module, "posts")

func init() {
	// Register module with fieldset handler for API access
	GlobalFieldsetHandler.RegisterModule(Module)
}

// Route handlers using the fieldset system
func List(w http.ResponseWriter, r *http.Request) {
	controller.List(w, r)
}

func Create(w http.ResponseWriter, r *http.Request) {
	controller.Create(w, r)
}

func View(w http.ResponseWriter, r *http.Request) {
	controller.View(w, r)
}

func Edit(w http.ResponseWriter, r *http.Request) {
	controller.Edit(w, r)
}

func Delete(w http.ResponseWriter, r *http.Request) {
	controller.Delete(w, r)
}
