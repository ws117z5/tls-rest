package posts

import (
	"tls-rest/go/lib/module"
)

// Posts module following CInvoice paradigm
type Posts struct {
	*module.ModuleAbstract[interface{}]
}

// NewPosts creates a new Posts module instance
func NewPosts() *Posts {
	moduleInstance := &Posts{
		ModuleAbstract: &module.ModuleAbstract[interface{}]{
			ID:                "posts",
			Name:              "Posts",
			Rights:            make(map[int]int),
			DefaultPermission: 1, // PERMISSION_READ - Posts module has read access by default
		},
	}

	// Initialize fields in dedicated method (like PHP init() method)
	moduleInstance.init()

	return moduleInstance
}

// init method defines all fields and module configuration (like CInvoice.php init method)
func (p *Posts) init() {
	// Define module-specific fields (default fields auto-added by system)
	fields := []module.Field{
		module.NewField("title", module.TYPE_STRING, true).
			WithLabel("Title").
			WithDescription("Post title").
			WithValidation("minLength", 3).
			WithValidation("maxLength", 200),

		module.NewField("images", module.TYPE_JSON, false).
			WithLabel("Images").
			WithDescription("Post images array").
			NonSortable().
			NonSearchable(),

		module.NewField("content", module.TYPE_TEXT, true).
			WithLabel("Content").
			WithDescription("Post content").
			WithValidation("minLength", 10).
			NonSortable(),

		module.NewField("public", module.TYPE_CHECKBOX, false).
			WithLabel("Public").
			WithDescription("Whether the post is publicly visible").
			WithDefault(false),
	}

	p.ModuleAbstract.Fields = fields
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
