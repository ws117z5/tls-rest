package pgdb

import (
	"context"
	"fmt"
	"net"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"tls-rest/go/constants"
	"tls-rest/go/engine/controllers/log"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// This package wraps pgx v5 (github.com/jackc/pgx/v5). pgx speaks PostgreSQL's
// native protocol and uses $1/$2 placeholders directly, so raw SQL written with
// $N — in this layer, in the fieldset engine, and in the auth/pages code — binds
// correctly with no placeholder translation. (The previous go-pg driver used ?
// placeholders, which is why $N queries failed with "there is no parameter $1".)

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

// A single shared connection pool is created lazily and reused across all
// GetInstance() calls. pgxpool is safe for concurrent use; each GetInstance
// returns a lightweight *Db wrapper over the same pool.
var (
	sharedPool *pgxpool.Pool
	poolOnce   sync.Once
	poolErr    error
)

func dsn() string {
	// Host/port/sslmode come from the environment so the same binary connects to
	// a local Postgres in dev and a managed instance (often sslmode=require) in
	// production. PG_ADDR may be "host" or "host:port"; PG_SSLMODE defaults to
	// disable for local dev.
	host, port := constants.Env("PG_HOST", "localhost"), constants.Env("PG_PORT", "5432")
	if addr := constants.PDb.Addr; addr != "" {
		if h, p, err := net.SplitHostPort(addr); err == nil {
			host, port = h, p
		} else {
			host = addr
		}
	}
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, constants.PDb.User, constants.PDb.Password, constants.PDb.Database,
		constants.Env("PG_SSLMODE", "disable"),
	)
}

func getPool() (*pgxpool.Pool, error) {
	poolOnce.Do(func() {
		sharedPool, poolErr = pgxpool.New(context.Background(), dsn())
	})
	return sharedPool, poolErr
}

// GetInstance returns a *Db backed by the shared pgx pool.
func GetInstance() (*Db, error) {
	pool, err := getPool()
	if err != nil {
		return nil, err
	}
	return &Db{pool: pool}, nil
}

type DefaultDb struct {
	ID        int64  `db:"id"`
	UUID      string `db:"uuid"`
	CreatedAt string `db:"created_at"`
	UpdatedAt string `db:"updated_at"`
}

type Db struct {
	pool         *pgxpool.Pool
	queriesCount int
	queriesTime  float64

	LastInsertId int64
}

// NewDb builds a *Db from a pgx connection string ("postgres://…" or keyword
// "host=… user=…"). Returns an error if the pool cannot be created.
func NewDb(connString string) (*Db, error) {
	pool, err := pgxpool.New(context.Background(), connString)
	if err != nil {
		return nil, err
	}
	return &Db{pool: pool}, nil
}

func (db *Db) track(start time.Time) {
	db.queriesCount++
	db.queriesTime += time.Since(start).Seconds()
}

// Close releases the shared pool. Note the pool is shared across all *Db
// wrappers; closing it affects every caller.
func (db *Db) Close() error {
	db.pool.Close()
	return nil
}

// Query executes a statement that returns no rows (INSERT/UPDATE/DELETE/DDL) and
// returns the command tag. Alias kept for callers that used the old name.
func (db *Db) Query(query string, args ...interface{}) (pgconn.CommandTag, error) {
	return db.Exec(query, args...)
}

// Exec executes a statement that returns no rows and returns the command tag.
func (db *Db) Exec(query string, args ...interface{}) (pgconn.CommandTag, error) {
	start := time.Now()
	tag, err := db.pool.Exec(context.Background(), query, args...)
	db.track(start)
	if err != nil {
		log.Printf("SQL error: %s\nError: %v", query, err)
		return tag, err
	}
	return tag, nil
}

// RQuery runs a query and returns the rows as a slice of column->value maps.
func (db *Db) RQuery(query string, args ...interface{}) ([]map[string]interface{}, error) {
	start := time.Now()
	rows, err := db.pool.Query(context.Background(), query, args...)
	if err != nil {
		db.track(start)
		log.Printf("SQL error: %s\nError: %v", query, err)
		return nil, err
	}
	results, err := pgx.CollectRows(rows, pgx.RowToMap)
	db.track(start)
	if err != nil {
		log.Printf("SQL error: %s\nError: %v", query, err)
		return nil, err
	}
	return results, nil
}

// GetInsertID returns the id of the most recent InsertRow (pgx has no implicit
// LastInsertId; InsertRow uses RETURNING id and records it here).
func (db *Db) GetInsertID(_ pgconn.CommandTag) (int64, error) {
	return db.LastInsertId, nil
}

// GetAffectedRows returns the number of rows affected by an Exec/Query result.
func (db *Db) GetAffectedRows(result pgconn.CommandTag) (int64, error) {
	return result.RowsAffected(), nil
}

// GetAll fetches all rows as a slice of maps.
func (db *Db) GetAll(query string, args ...interface{}) ([]map[string]interface{}, error) {
	return db.RQuery(query, args...)
}

// GetOne fetches the first row as a map. Returns an error if no rows match.
// GetOne fetches the first row as a column->value map, or nil when no row
// matches (nil map, nil error — callers check for a nil result).
func (db *Db) GetOne(query string, args ...interface{}) (map[string]interface{}, error) {
	results, err := db.RQuery(query, args...)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}
	return results[0], nil
}

// Quote safely quotes a (possibly dotted) table or column identifier.
func (db *Db) Quote(name string) string {
	parts := strings.Split(name, ".")
	for i, part := range parts {
		parts[i] = `"` + part + `"`
	}
	return strings.Join(parts, ".")
}

// Escape escapes a value for safe use inside a single-quoted SQL literal. Prefer
// parameter binding ($N) over this wherever possible.
func (db *Db) Escape(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}

// InsertRow inserts a row and returns its id (via RETURNING id).
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

	start := time.Now()
	var id int64
	err := db.pool.QueryRow(context.Background(), query, values...).Scan(&id)
	db.track(start)
	if err != nil {
		log.Printf("SQL error: %s\nError: %v", query, err)
		return 0, err
	}

	db.LastInsertId = id
	return id, nil
}

// UpdateRow updates a row identified by keyField=keyValue and returns the number
// of affected rows.
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

	tag, err := db.Exec(query, values...)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// DeleteRow deletes rows matching keyField=keyValue and returns the number
// affected.
func (db *Db) DeleteRow(table string, keyField string, keyValue interface{}) (int64, error) {
	query := fmt.Sprintf("DELETE FROM %s WHERE %s = $1",
		db.Quote(table),
		db.Quote(keyField),
	)
	tag, err := db.Exec(query, keyValue)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// TableExists reports whether a table exists in the public schema.
func (db *Db) TableExists(tableName string) (bool, error) {
	query := `SELECT EXISTS (
		SELECT 1 FROM information_schema.tables
		WHERE table_schema = 'public'
		AND table_name = $1
	)`
	var exists bool
	err := db.pool.QueryRow(context.Background(), query, tableName).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check if table exists: %w", err)
	}
	return exists, nil
}

// GetQueriesCount returns the number of executed queries.
func (db *Db) GetQueriesCount() int {
	return db.queriesCount
}

// GetQueriesTime returns the total time spent on queries.
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
			continue
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
			continue
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
	// Concrete types not distinguished by Kind alone.
	switch t {
	case reflect.TypeOf([]byte(nil)):
		return coerceBytes(value), nil
	case reflect.TypeOf(time.Time{}):
		if tm, ok := value.(time.Time); ok {
			return tm, nil
		}
		return time.Time{}, nil
	}

	switch t.Kind() {
	case reflect.String:
		switch v := value.(type) {
		case string:
			return v, nil
		case []byte:
			return string(v), nil
		case nil:
			return "", nil
		case fmt.Stringer:
			return v.String(), nil
		default:
			return fmt.Sprintf("%v", v), nil
		}
	case reflect.Int:
		return int(coerceInt64(value)), nil
	case reflect.Int32:
		return int32(coerceInt64(value)), nil
	case reflect.Int64:
		return coerceInt64(value), nil
	case reflect.Float64:
		switch v := value.(type) {
		case float64:
			return v, nil
		case float32:
			return float64(v), nil
		case int64:
			return float64(v), nil
		case int:
			return float64(v), nil
		case string:
			f, _ := strconv.ParseFloat(v, 64)
			return f, nil
		}
		return float64(0), nil
	case reflect.Bool:
		switch v := value.(type) {
		case bool:
			return v, nil
		case string:
			b, _ := strconv.ParseBool(v)
			return b, nil
		}
		return false, nil
	case reflect.Slice:
		if t.Elem().Kind() == reflect.Uint8 {
			return coerceBytes(value), nil
		}
	}
	return value, nil
}

// Coerce converts a DB result value to T using CoerceValue (the single coercion
// authority). It returns the zero value of T when the value can't be converted,
// and is the ergonomic way to read a map value from RQuery/GetOne/GetAll as a
// concrete Go type, e.g. Coerce[int64](row["id"]) or Coerce[string](row["name"]).
func Coerce[T any](v interface{}) T {
	var zero T
	out, err := CoerceValue(v, reflect.TypeOf(zero))
	if err != nil {
		return zero
	}
	if typed, ok := out.(T); ok {
		return typed
	}
	return zero
}

func coerceInt64(v interface{}) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case int32:
		return int64(n)
	case float64:
		return int64(n)
	case string:
		x, _ := strconv.ParseInt(n, 10, 64)
		return x
	}
	return 0
}

func coerceBytes(v interface{}) []byte {
	switch b := v.(type) {
	case []byte:
		return b
	case string:
		return []byte(b)
	}
	return nil
}
