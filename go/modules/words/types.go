package words

import (
	"time"

	. "tls-rest/go/engine/controllers/module"
)

// WordObj is the stored shape of a word row. translations and win_rate are NOT
// here: they are calculated (virtual) fields, computed in SQL at read time.
type WordObj struct {
	ID        int64     `json:"id" db:"id"`
	UUID      string    `json:"uuid" db:"uuid"`
	Word      string    `json:"word" db:"word"`
	Tries     int       `json:"tries" db:"tries"`
	Success   int       `json:"success" db:"success"`
	Fail      int       `json:"fail" db:"fail"`
	Created   time.Time `json:"created" db:"created"`
	Updated   time.Time `json:"updated" db:"updated"`
	CreatedBy int       `json:"created_by" db:"created_by"`
}

func (WordObj) TableName() string { return "words" }

// Words is a per-user vocabulary module.
type Words struct {
	*ModuleAbstract[interface{}]
}
