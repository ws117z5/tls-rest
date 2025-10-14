package mysql

import (
	"database/sql"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/ws117z5/tls-rest/go/lib/log"
)

type Db struct {
	conn         *sql.DB
	queriesCount int
	queriesTime  float64
}

// NewDb initializes a new database connection
func GetInstance(dsn string) (*Db, error) {
	conn, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	return &Db{conn: conn}, nil
}

// Close closes the database connection
func (db *Db) Close() error {
	return db.conn.Close()
}

// Query executes a query and returns the result
func (db *Db) Query(query string, args ...interface{}) (*sql.Rows, error) {
	startTime := time.Now()
	rows, err := db.conn.Query(query, args...)
	db.queriesCount++
	db.queriesTime += time.Since(startTime).Seconds()
	if err != nil {
		log.Printf("SQL error: %s\nError: %v", query, err)
		return nil, err
	}
	return rows, nil
}

// Exec executes a query without returning rows
func (db *Db) Exec(query string, args ...interface{}) (sql.Result, error) {
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

// GetInsertID returns the last inserted ID
func (db *Db) GetInsertID(result sql.Result) (int64, error) {
	return result.LastInsertId()
}

// GetAffectedRows returns the number of affected rows
func (db *Db) GetAffectedRows(result sql.Result) (int64, error) {
	return result.RowsAffected()
}

// GetAll fetches all rows as a slice of maps
func (db *Db) GetAll(query string, args ...interface{}) ([]map[string]interface{}, error) {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}
	for rows.Next() {
		row := make([]interface{}, len(columns))
		rowPointers := make([]interface{}, len(columns))
		for i := range row {
			rowPointers[i] = &row[i]
		}

		if err := rows.Scan(rowPointers...); err != nil {
			return nil, err
		}

		result := make(map[string]interface{})
		for i, col := range columns {
			result[col] = row[i]
		}
		results = append(results, result)
	}

	return results, nil
}

// GetOne fetches a single value
func (db *Db) GetOne(query string, args ...interface{}) (interface{}, error) {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if rows.Next() {
		var value interface{}
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		return value, nil
	}

	return nil, nil
}

// Quote safely quotes a table or column name
func (db *Db) Quote(name string) string {
	parts := strings.Split(name, ".")
	for i, part := range parts {
		parts[i] = "`" + part + "`"
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

	for field, value := range fieldValues {
		fields = append(fields, db.Quote(field))
		values = append(values, value)
		placeholders = append(placeholders, "?")
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		db.Quote(table),
		strings.Join(fields, ", "),
		strings.Join(placeholders, ", "),
	)

	result, err := db.Exec(query, values...)
	if err != nil {
		return 0, err
	}

	return db.GetInsertID(result)
}

// UpdateRow updates a row in a table
func (db *Db) UpdateRow(table string, fieldValues map[string]interface{}, keyField string, keyValue interface{}) (int64, error) {
	setClauses := []string{}
	values := []interface{}{}

	for field, value := range fieldValues {
		setClauses = append(setClauses, fmt.Sprintf("%s = ?", db.Quote(field)))
		values = append(values, value)
	}
	values = append(values, keyValue)

	query := fmt.Sprintf("UPDATE %s SET %s WHERE %s = ?",
		db.Quote(table),
		strings.Join(setClauses, ", "),
		db.Quote(keyField),
	)

	result, err := db.Exec(query, values...)
	if err != nil {
		return 0, err
	}

	return db.GetAffectedRows(result)
}

// DeleteRow deletes a row from a table
func (db *Db) DeleteRow(table string, keyField string, keyValue interface{}) (int64, error) {
	query := fmt.Sprintf("DELETE FROM %s WHERE %s = ?",
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
