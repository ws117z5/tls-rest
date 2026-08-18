package posts

import (
	. "tls-rest/go/engine"
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

		NewField("images", TYPE_IMAGE, false).
			WithLabel("Images").
			WithDescription("Post images").
			WithOption("multiple", true).
			NonSortable().
			NonSearchable(),

		NewField("content", TYPE_TEXT, true).
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
