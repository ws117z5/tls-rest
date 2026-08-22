package posts

import (
	"time"

	. "tls-rest/go/engine/controllers/module"
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
