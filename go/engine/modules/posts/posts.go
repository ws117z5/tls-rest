package posts

import (
	module "tls-rest/go/engine"
)

// Posts module following CInvoice paradigm
type Posts struct {
	*module.ModuleAbstract[interface{}]
}

// NewPosts creates a new Posts module instance
func NewPosts() *Posts {
	moduleInstance := &Posts{
		ModuleAbstract: &module.ModuleAbstract[interface{}]{
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
	moduleInstance.ModuleAbstract.Fields = moduleInstance.fieldset()
	moduleInstance.ModuleAbstract.Filters = moduleInstance.filters()

	return moduleInstance
}

// fieldset defines the module's fields. It is the field-set counterpart to
// filters() in filters.go: both build a set that the fieldset engine consumes.
func (p *Posts) fieldset() []module.Field {
	return []module.Field{
		module.NewField("title", module.TYPE_STRING, true).
			WithLabel("Title").
			WithDescription("Post title").
			WithValidation("minLength", 3).
			WithValidation("maxLength", 200).
			WithOption("width", "600px"),

		module.NewField("images", module.TYPE_IMAGE, false).
			WithLabel("Images").
			WithDescription("Post images").
			WithOption("multiple", true).
			NonSortable().
			NonSearchable(),

		module.NewField("content", module.TYPE_TEXT, true).
			WithLabel("Content").
			WithDescription("Post content").
			WithValidation("minLength", 10).
			WithOption("width", "600px").
			WithOption("height", "300px").
			NonSortable(),

		module.NewField("public", module.TYPE_CHECKBOX, false).
			WithLabel("Public").
			WithDescription("Whether the post is publicly visible").
			WithDefault(false),
	}
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
func init() {
	// Create module instance
	Module = NewPosts()

	// Initialize with database table - routes are automatically registered
	Module.Initialize("posts")
}
