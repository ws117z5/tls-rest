package posts

import (
	. "tls-rest/go/engine/controllers/field"
	. "tls-rest/go/engine/controllers/module"
)

// fieldset defines the module's fields. It is the field-set counterpart to
// filters() in filters.go: both build a set that the fieldset engine consumes.
func (p *Posts) fieldset() []Field {
	return []Field{

		NewField("title", TYPE_STRING, true).
			WithLabel("Title").
			WithDescription("Post title").
			WithValidation("minLength", 3).
			WithValidation("maxLength", 200).
			WithOption("width", "600px"),

		NewField("author", TYPE_STRING, true).
			WithLabel("Author").
			WithValidation("minLength", 3).
			WithValidation("maxLength", 200).
			WithOption("width", "600px").
			WithSQL("(SELECT CONCAT(first_name, ' ', last_name) FROM users WHERE users.id = posts.created_by LIMIT 1)").
			AsVirtual(),

		NewField("images", TYPE_IMAGE, false).
			WithLabel("Images").
			WithDescription("Post images").
			WithOption("multiple", true).
			NonSortable().
			NonSearchable(),

		NewField("content", TYPE_MARKDOWN, true).
			WithLabel("Content").
			WithDescription("Post content").
			WithValidation("minLength", 10).
			WithOption("width", "600px").
			WithOption("height", "300px").
			NonSortable(),

		NewField("public", TYPE_CHECKBOX, false).
			WithLabel("Public").
			WithDescription("Whether the post is publicly visible").
			WithDefault(false),
	}
}

// NewPosts creates a new Posts module instance
func NewPosts() *Posts {
	m := &Posts{
		ModuleAbstract: &ModuleAbstract[interface{}]{
			ID:     "posts",
			Name:   "Posts",
			Rights: make(map[int]int),
			// Public module: everyone may read (list/view); writes require rights.
			DefaultPermission:    1, // PERMISSION_READ
			DefaultPermissionSet: true,
		},
	}

	// Build the field and filter sets. fieldset() defines the columns/form
	// fields; filters() (in filters.go) defines the list-mode filters. Default
	// system fields are added automatically by Initialize().
	m.ModuleAbstract.Fields = m.fieldset()
	m.ModuleAbstract.Filters = m.filters()

	m.Initialize("posts")

	return m
}

// Global module instance (initialized at startup)
var Module *Posts

// Package initialization (similar to PHP module registration)
// Routes are automatically registered when Initialize() is called
// No manual route registration needed - the module system handles:
// GET    /posts     -> List()
// POST   /posts     -> Create()
// GET    /posts/{id} -> View()
// PUT    /posts/{id} -> Edit()
// DELETE /posts/{id} -> Delete()
func Init() {
	// Create module instance
	Module = NewPosts()
}
