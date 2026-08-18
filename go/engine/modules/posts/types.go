package posts

import (
	"time"

	. "tls-rest/go/engine"
)

// PostObj represents the data structure for posts
type PostObj struct {
	ID        int64     `json:"id" db:"id"`
	UUID      string    `json:"uuid" db:"uuid"`
	Title     string    `json:"title" db:"title"`
	Images    []string  `json:"images" db:"images"`
	Content   string    `json:"content" db:"content"`
	Created   time.Time `json:"created" db:"created"`
	Updated   time.Time `json:"updated" db:"updated"`
	CreatedBy int       `json:"created_by" db:"created_by"`
}

// TableName returns the database table name
func (PostObj) TableName() string {
	return "posts"
}

// Posts module following CInvoice paradigm
type Posts struct {
	*ModuleAbstract[interface{}]
}

// NewPosts creates a new Posts module instance
func NewPosts() *Posts {
	moduleInstance := &Posts{
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
	moduleInstance.ModuleAbstract.Fields = moduleInstance.fieldset()
	moduleInstance.ModuleAbstract.Filters = moduleInstance.filters()

	return moduleInstance
}
