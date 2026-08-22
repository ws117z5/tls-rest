package db

import (
	"fmt"
	"log"
)

//postgre "github.com/ws117z5/tls-rest/lib/db"
//redis github.com/ws117z5/tls-rest/lib/db"

type Varchar struct {
	Value string `db:"varchar"`
}

type Integer struct {
	Value int64 `db:"integer"`
}

type Float struct {
	Value float64 `db:"float"`
}

// DateTime is a custom type for handling date and time in the database
type DateTime struct {
	Value string `db:"datetime"`
}

func init() {

}

type Db interface {
	Client() interface{}
	Database() interface{}

	_GetAll(query string, args ...interface{}) ([]map[string]interface{}, error)
	GetAll(table string, fields []string, where map[string]string) map[string]string
	InsertRow(table string, fieldValues map[string]interface{}) (int64, error)
	DeleteRow(table string, keyField string, keyValue interface{}) (int64, error)
	UpdateRow(table string, fieldValues map[string]interface{}, keyField string, keyValue interface{}) (int64, error)
	GetOne(table string, fields []string, where map[string]string) (map[string]string, error)
	Query(query string, args ...interface{}) ([]map[string]string, error)

	Init()
}

func Init(some interface{}) {
	// does the passed value honor the 'Printer' interface
	if v, ok := some.(Db); ok {
		// yes - so Print()!
		v.Init()
	} else {
		log.Printf("value of type %T passed has no Init() method.\n", some)
		return
	}
}

func GetAll(some interface{}, table string, fields []string, where map[string]string) (map[string]string, interface{}) {
	if v, ok := some.(Db); ok {
		// yes - so Print()!
		return v.GetAll(table, fields, where), nil
	}
	err := fmt.Sprintf("value of type %T passed has no GetAll() method.\n", some)
	return nil, err
}
