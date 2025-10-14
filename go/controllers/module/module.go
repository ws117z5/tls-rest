package module

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/ws117z5/tls-rest/go/lib"
	"github.com/ws117z5/tls-rest/go/lib/auth"
	"github.com/ws117z5/tls-rest/go/lib/db/pgdb"

	"github.com/go-pg/pg/v10"
	"github.com/go-pg/urlstruct"
	"github.com/gorilla/mux"
)

type Module struct {
	ID        int64     `json:"id"`
	UUID      string    `sql:"default:uuid_generate_v4()" json:"uuid"`
	Name      string    `json:"name"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `sql:"default:now()" json:"created_at"`
	UpdatedAt time.Time `sql:"default:now()" json:"updated_at"`
}

type Filter struct {
	//nolint
	tableName struct{} `sql:"modules" urlstruct:"b"`

	urlstruct.Pager
}

type Data struct {
	Fieldset map[string]string
	Data     []Module
}

// Helper: Build WHERE clause from query params and struct fields
func applyWhereFromQuery(q *pg.Query, params map[string][]string, model interface{}) *pg.Query {
	modelType := reflect.TypeOf(model)
	if modelType.Kind() == reflect.Ptr {
		modelType = modelType.Elem()
	}
	for key, values := range params {
		if len(values) == 0 {
			continue
		}
		// Check if key matches a struct field (by json tag or field name)
		for i := 0; i < modelType.NumField(); i++ {
			field := modelType.Field(i)
			jsonTag := field.Tag.Get("json")
			if strings.EqualFold(field.Name, key) || (jsonTag != "" && strings.EqualFold(jsonTag, key)) {
				q = q.Where(fmt.Sprintf("%s = ?", key), values[0])
				break
			}
		}
	}
	return q
}

func List(w http.ResponseWriter, r *http.Request) {
	//fmt.Fprintln(w, "List Posts!")
	session, ok := r.Context().Value(auth.SESSION_KEY).(*auth.ContextKey)
	if ok && session != nil {
		// Use session.ID, session.UserID, etc.
	}
	//vars := mux.Vars(r)

	modules := make([]Module, 0)
	//var values = pager.Values(r.URL.Query())

	filter := new(Filter)
	ctx := new(context.Context)
	keys := r.URL.Query()
	err := urlstruct.Unmarshal(*ctx, keys, filter)
	if err != nil {
		panic(err)
	}

	db, _ := pgdb.GetInstance()
	q := db.Model(&modules)
	q = applyWhereFromQuery(q, r.URL.Query(), Module{})
	err = q.
		Limit(filter.Pager.GetLimit()).
		Offset(filter.Pager.GetOffset()).
		Select()

	if err != nil {
		log.Println(err)
	}

	err = json.NewEncoder(w).Encode(Data{lib.GetFields(Module{}), modules})
	if err != nil {
		log.Panic(err)
		fmt.Fprintln(w, "List Posts!")
	}
}

func Create(w http.ResponseWriter, r *http.Request) {
	var module Module
	err := json.NewDecoder(r.Body).Decode(&module)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	db, _ := pgdb.GetInstance()
	_, err = db.Model(&module).Insert()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	err = json.NewEncoder(w).Encode(module)
	if err != nil {
		log.Panic(err)
		fmt.Fprintln(w, "Error encoding the response")
	}
}

func View(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	db, _ := pgdb.GetInstance()
	module := new(Module)
	err := db.Model(module).Where("id = ?", vars["id"]).Select()
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	err = json.NewEncoder(w).Encode(module)
	if err != nil {
		log.Panic(err)
		fmt.Fprintln(w, "Error encoding the response")
	}
}

func Edit(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	moduleId := vars["id"]

	var module Module
	err := json.NewDecoder(r.Body).Decode(&module)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	db, _ := pgdb.GetInstance()
	module.ID = 0 // Reset ID to insert as new record
	_, err = db.Model(&module).Where("id = ?", moduleId).Update()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(module)
	if err != nil {
		log.Panic(err)
		fmt.Fprintln(w, "Error encoding the response")
	}
}

func Delete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	moduleId := vars["id"]

	db, _ := pgdb.GetInstance()
	module := new(Module)
	_, err := db.Model(module).Where("id = ?", moduleId).Delete()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
	fmt.Fprintln(w, "Module deleted successfully")
}
