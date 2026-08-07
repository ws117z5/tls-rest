package pgdb

import (
	//"github.com/lib/pq"
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"tls-rest/go/constants"

	"tls-rest/go/lib/log"

	"github.com/go-pg/pg/v10"
)

type DbLogEvent struct {
	Timestamp    time.Time     `json:"timestamp"`
	Operation    string        `json:"operation"`
	Table        string        `json:"table,omitempty"`
	Query        string        `json:"query"`
	Args         []interface{} `json:"args,omitempty"`
	Duration     float64       `json:"duration_ms"`
	RowsAffected int64         `json:"rows_affected,omitempty"`
	Error        string        `json:"error,omitempty"`
	Success      bool          `json:"success"`
}

// GetInstance returns an instance of pgDb
func GetInstance() (*Db, error) {
	db := pg.Connect(&pg.Options{
		//Addr: config.PDb.Addr,
		Database: constants.PDb.Database,
		User:     constants.PDb.User,
		Password: constants.PDb.Password,
	})

	ctx := context.Background()

	err := db.Ping(ctx)

	return &Db{conn: db}, err
}

type DefaultDb struct {
	ID        int64  `db:"id"`
	UUID      string `db:"uuid"`
	CreatedAt string `db:"created_at"`
	UpdatedAt string `db:"updated_at"`
}

type Db struct {
	conn         *pg.DB
	queriesCount int
	queriesTime  float64

	LastInsertId int64
}

// NewDb initializes a new database connection
func NewDb(opts *pg.Options) *Db {
	conn := pg.Connect(opts)
	return &Db{conn: conn}
}

// Close closes the database connection
func (db *Db) Close() error {
	return db.conn.Close()
}

func (db *Db) Model(model ...interface{}) *pg.Query {
	// Forward the caller's model(s) to go-pg. The variadic must be spread with
	// `model...`; and db.conn must NOT be passed as a model — doing so made go-pg
	// derive the table name from *pg.DB (type "DB" -> "dbs"), producing
	// `relation "dbs" does not exist` for every query.
	return db.conn.Model(model...)
}

// Query executes a query and returns the result
func (db *Db) Query(query string, args ...interface{}) (pg.Result, error) {
	startTime := time.Now()
	result, err := db.conn.Exec(query, args...)
	db.queriesCount++
	db.queriesTime += time.Since(startTime).Seconds()
	if err != nil {
		log.Printf("SQL error: %s\nError: %v", query, err)
		return nil, err
	}
	return result, nil
}

func (db *Db) RQuery(query string, args ...interface{}) ([]map[string]interface{}, error) {
	startTime := time.Now()
	var results []map[string]interface{}
	_, err := db.conn.Query(&results, query, args...)
	db.queriesCount++
	db.queriesTime += time.Since(startTime).Seconds()
	if err != nil {
		log.Printf("SQL error: %s\nError: %v", query, err)
		return nil, err
	}
	return results, nil
}

// func (db *Db) QueryResults(res pg.Result) ([]map[string]interface{}, error) {
// 	var results []map[string]interface{}
// 	_, err := db.conn.Query(&results, pg.Scan(pg.ResultRows(pg)))
// 	if err != nil {
// 		log.Printf("SQL error: %v", err)
// 		return nil, err
// 	}
// 	return results, nil
// }

// todo add support for inserting rows with returning ID
// Exec executes a query without returning rows
func (db *Db) Exec(query string, args ...interface{}) (pg.Result, error) {
	startTime := time.Now()
	result, err := db.conn.Exec(query, args...)
	db.queriesCount++
	db.queriesTime += time.Since(startTime).Seconds()
	if err != nil {
		log.Printf("SQL error: %s\nError: %v", query, err)
		return nil, err
	}
	return result, nil
}

// GetInsertID returns the last inserted ID (Postgres: use RETURNING id in your query)
func (db *Db) GetInsertID(result pg.Result) (int64, error) {
	// Not directly supported; use RETURNING in your insert query
	return db.LastInsertId, nil
}

// GetAffectedRows returns the number of affected rows
func (db *Db) GetAffectedRows(result pg.Result) (int64, error) {
	return int64(result.RowsAffected()), nil
}

// GetAll fetches all rows as a slice of maps
func (db *Db) GetAll(query string, args ...interface{}) ([]map[string]interface{}, error) {
	//var results []map[string]interface{}
	results, err := db.RQuery(query, args...)
	if err != nil {
		return nil, err
	}

	return results, nil
}

// GetOne fetches a single value
func (db *Db) GetOne(query string, args ...interface{}) (interface{}, error) {
	results, err := db.RQuery(query, args...)
	if err != nil {
		return nil, err
	}
	return results[0], nil
}

// Quote safely quotes a table or column name
func (db *Db) Quote(name string) string {
	parts := strings.Split(name, ".")
	for i, part := range parts {
		parts[i] = `"` + part + `"`
	}
	return strings.Join(parts, ".")
}

// Escape escapes a value for safe use in queries
func (db *Db) Escape(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}

// InsertRow inserts a row into a table
func (db *Db) InsertRow(table string, fieldValues map[string]interface{}) (int64, error) {
	fields := []string{}
	values := []interface{}{}
	placeholders := []string{}

	i := 1
	for field, value := range fieldValues {
		fields = append(fields, db.Quote(field))
		values = append(values, value)
		placeholders = append(placeholders, fmt.Sprintf("$%d", i))
		i++
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) RETURNING id",
		db.Quote(table),
		strings.Join(fields, ", "),
		strings.Join(placeholders, ", "),
	)

	var id int64
	_, err := db.conn.QueryOne(pg.Scan(&id), query, values...)
	if err != nil {
		return 0, err
	}

	db.LastInsertId = id
	return id, nil
}

// UpdateRow updates a row in a table
func (db *Db) UpdateRow(table string, fieldValues map[string]interface{}, keyField string, keyValue interface{}) (int64, error) {
	setClauses := []string{}
	values := []interface{}{}
	i := 1

	for field, value := range fieldValues {
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", db.Quote(field), i))
		values = append(values, value)
		i++
	}
	values = append(values, keyValue)
	query := fmt.Sprintf("UPDATE %s SET %s WHERE %s = $%d",
		db.Quote(table),
		strings.Join(setClauses, ", "),
		db.Quote(keyField),
		i,
	)

	result, err := db.Exec(query, values...)
	if err != nil {
		return 0, err
	}
	return db.GetAffectedRows(result)
}

// DeleteRow deletes a row from a table
func (db *Db) DeleteRow(table string, keyField string, keyValue interface{}) (int64, error) {
	query := fmt.Sprintf("DELETE FROM %s WHERE %s = $1",
		db.Quote(table),
		db.Quote(keyField),
	)
	result, err := db.Exec(query, keyValue)
	if err != nil {
		return 0, err
	}
	return db.GetAffectedRows(result)
}

// GetQueriesCount returns the number of executed queries
func (db *Db) GetQueriesCount() int {
	return db.queriesCount
}

// GetQueriesTime returns the total time spent on queries
func (db *Db) GetQueriesTime() float64 {
	return db.queriesTime
}

// --- Type coercion helpers (same as MySQL version) ---

func CoerceByType(data map[string]interface{}, current *map[string]interface{}, schemaType reflect.Type) error {
	for i := 0; i < schemaType.NumField(); i++ {
		field := schemaType.Field(i)
		key := field.Tag.Get("db")
		if key == "" {
			key = strings.ToLower(field.Name)
		}
		expectedType := field.Type

		raw, ok := data[key]
		if !ok {
			log.Warningf("Missing value for field %s", key)
			continue // missing value is skipped
		}

		val, err := CoerceValue(raw, expectedType)
		if err != nil {
			return fmt.Errorf("coercing field %s: %w", key, err)
		}

		(*current)[key] = val
	}

	return nil
}

func CoerceToSchema[T any](data map[string]interface{}) (map[string]interface{}, error) {
	var schema T

	schemaType := reflect.TypeOf(schema)

	coerced := make(map[string]interface{})

	for i := 0; i < schemaType.NumField(); i++ {
		field := schemaType.Field(i)
		// Handle embedded struct recursively
		if field.Anonymous && field.Type.Kind() == reflect.Struct {
			if err := CoerceByType(data, &coerced, field.Type); err != nil {
				return nil, err
			}
			continue
		}

		key := field.Tag.Get("db")
		if key == "" {
			key = strings.ToLower(field.Name)
		}
		expectedType := field.Type

		raw, ok := data[key]
		if !ok {
			continue // missing value is skipped
		}

		val, err := CoerceValue(raw, expectedType)
		if err != nil {
			return nil, fmt.Errorf("coercing field %s: %w", key, err)
		}

		coerced[key] = val
	}

	return coerced, nil
}

func CoerceValue(value interface{}, t reflect.Type) (interface{}, error) {
	switch t.Kind() {
	case reflect.String:
		switch v := value.(type) {
		case string:
			return v, nil
		case []byte:
			return string(v), nil
		case fmt.Stringer:
			return v.String(), nil
		default:
			return fmt.Sprintf("%v", v), nil
		}
	case reflect.Int, reflect.Int64:
		switch v := value.(type) {
		case int:
			return v, nil
		case int64:
			return int(v), nil
		case float64:
			return int(v), nil
		case string:
			return strconv.Atoi(v)
		}
	case reflect.Float64:
		switch v := value.(type) {
		case float64:
			return v, nil
		case string:
			return strconv.ParseFloat(v, 64)
		}
	case reflect.Bool:
		switch v := value.(type) {
		case bool:
			return v, nil
		case string:
			return strconv.ParseBool(v)
		}
	}
	return value, nil
}

// TableExists checks if a table exists in the database
func (db *Db) TableExists(tableName string) (bool, error) {
	query := `SELECT EXISTS (
		SELECT 1 FROM information_schema.tables 
		WHERE table_schema = 'public' 
		AND table_name = ?
	)`

	var exists bool
	_, err := db.conn.QueryOne(pg.Scan(&exists), query, tableName)
	if err != nil {
		return false, fmt.Errorf("failed to check if table exists: %w", err)
	}

	return exists, nil
}
