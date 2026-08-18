package modulerights

import (
	"time"
)

type UserGroupRight struct {
	tableName struct{} `pg:"user_group_rights"`

	ID      int64     `json:"id"`
	GroupID int64     `json:"group_id"`
	Module  string    `json:"module"`
	Modes   int       `json:"modes"`
	Created time.Time `sql:"default:now()" json:"created"`
	Updated time.Time `sql:"default:now()" json:"updated"`
}

// UserRight is one per-user grant.
type UserRight struct {
	tableName struct{} `pg:"user_rights"`

	ID      int64     `json:"id"`
	UserID  int64     `json:"user_id"`
	Module  string    `json:"module"`
	Modes   int       `json:"modes"`
	Created time.Time `sql:"default:now()" json:"created"`
	Updated time.Time `sql:"default:now()" json:"updated"`
}
