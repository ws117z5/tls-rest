package users

import (
	"time"
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
