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

	"tls-rest/go/lib"
	"tls-rest/go/lib/auth"
	"tls-rest/go/lib/db/pgdb"

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

// moduleColumns is the SELECT list for the modules table.
const moduleColumns = "id, uuid, name, created_by, created_at, updated_at"

// whereFromQuery builds a parameterised WHERE clause from query params, matching
// each param against a field of `model` (by field name or json tag). Only matched
// fields become conditions, and the column comes from the struct (json tag or
// lowercased field name) — never the raw query key — so this can't inject
// identifiers. Placeholders are Postgres-native $N.
func whereFromQuery(params map[string][]string, model interface{}) (string, []interface{}) {
	modelType := reflect.TypeOf(model)
	if modelType.Kind() == reflect.Ptr {
		modelType = modelType.Elem()
	}

	var clauses []string
	var args []interface{}
	i := 1

	for key, values := range params {
		if len(values) == 0 {
			continue
		}
		for f := 0; f < modelType.NumField(); f++ {
			field := modelType.Field(f)
			jsonTag := field.Tag.Get("json")
			if strings.EqualFold(field.Name, key) || (jsonTag != "" && strings.EqualFold(jsonTag, key)) {
				col := jsonTag
				if col == "" {
					col = strings.ToLower(field.Name)
				}
				clauses = append(clauses, fmt.Sprintf("%s = $%d", col, i))
				args = append(args, values[0])
				i++
				break
			}
		}
	}

	if len(clauses) == 0 {
		return "", nil
	}
	return strings.Join(clauses, " AND "), args
}

// rowToModule maps a result-set row (from RQuery) into a Module.
func rowToModule(row map[string]interface{}) Module {
	return Module{
		ID:        pgdb.AsInt64(row["id"]),
		UUID:      pgdb.AsString(row["uuid"]),
		Name:      pgdb.AsString(row["name"]),
		CreatedBy: pgdb.AsString(row["created_by"]),
		CreatedAt: pgdb.AsTime(row["created_at"]),
		UpdatedAt: pgdb.AsTime(row["updated_at"]),
	}
}

func List(w http.ResponseWriter, r *http.Request) {
	session, ok := r.Context().Value(auth.SESSION_KEY).(*auth.ContextKey)
	if ok && session != nil {
		// Use session.ID, session.UserID, etc.
	}

	modules := make([]Module, 0)

	filter := new(Filter)
	ctx := new(context.Context)
	keys := r.URL.Query()
	err := urlstruct.Unmarshal(*ctx, keys, filter)
	if err != nil {
		panic(err)
	}

	db, _ := pgdb.GetInstance()

	where, args := whereFromQuery(r.URL.Query(), Module{})
	query := "SELECT " + moduleColumns + " FROM modules"
	if where != "" {
		query += " WHERE " + where
	}
	query += fmt.Sprintf(" LIMIT %d OFFSET %d", filter.Pager.GetLimit(), filter.Pager.GetOffset())

	rows, err := db.RQuery(query, args...)
	if err != nil {
		log.Println(err)
	}
	for _, row := range rows {
		modules = append(modules, rowToModule(row))
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
	id, err := db.InsertRow("modules", map[string]interface{}{
		"name":       module.Name,
		"created_by": module.CreatedBy,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	module.ID = id

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
	rows, err := db.RQuery("SELECT "+moduleColumns+" FROM modules WHERE id = $1", vars["id"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(rows) == 0 {
		http.Error(w, "module not found", http.StatusNotFound)
		return
	}
	module := rowToModule(rows[0])

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
	_, err = db.UpdateRow("modules", map[string]interface{}{
		"name":       module.Name,
		"created_by": module.CreatedBy,
		"updated_at": time.Now(),
	}, "id", moduleId)
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
	_, err := db.DeleteRow("modules", "id", moduleId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
	fmt.Fprintln(w, "Module deleted successfully")
}
