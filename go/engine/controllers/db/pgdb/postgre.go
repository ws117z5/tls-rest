package pgdb

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"reflect"
	"strings"
	"sync"
	"time"

	"tls-rest/go/constants"
	"tls-rest/go/engine/controllers/log"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"uuid"
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
	ID      int64  `db:"id"`
	UUID    string `db:"uuid"`
	Created string `db:"created"`
	Updated string `db:"updated"`
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

// interpolate substitutes $1,$2,... with the actual argument values for LOGGING
// ONLY (never for execution). Replaces highest indices first so $11 isn't hit by
// the $1 pattern. Byte slices are summarised to avoid dumping large blobs.
func interpolate(query string, args []interface{}) string {
	out := query
	for i := len(args) - 1; i >= 0; i-- {
		out = strings.ReplaceAll(out, fmt.Sprintf("$%d", i+1), formatArg(args[i]))
	}
	return out
}

func formatArg(a interface{}) string {
	switch v := a.(type) {
	case nil:
		return "NULL"
	case string:
		return "'" + strings.ReplaceAll(v, "'", "''") + "'"
	case []byte:
		return fmt.Sprintf("[%d bytes]", len(v))
	case bool:
		return fmt.Sprintf("%t", v)
	default:
		return fmt.Sprintf("%v", v)
	}
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
		log.Printf("SQL error: %s\nError: %v", interpolate(query, args), err)
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
		log.Printf("SQL error: %s\nError: %v", interpolate(query, args), err)
		return nil, err
	}
	results, err := pgx.CollectRows(rows, pgx.RowToMap)
	db.track(start)
	if err != nil {
		log.Printf("SQL error: %s\nError: %v", interpolate(query, args), err)
		return nil, err
	}
	normalizeUUIDs(results)
	return results, nil
}

// normalizeUUIDs rewrites pgx's raw 16-byte UUID column values (any column, not
// just "uuid") to the canonical 8-4-4-4-12 string, so callers and JSON responses
// never see a byte array.
func normalizeUUIDs(rows []map[string]interface{}) {
	for _, row := range rows {
		for k, v := range row {
			if b, ok := v.([16]uint8); ok {
				row[k] = uuid.UUID(b).String()
			}
		}
	}
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

// GetAll fetches all rows as a slice of maps. UUID normalization is handled by
// RQuery.
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
// encodeParam prepares a Go value for binding to a SQL parameter. Values that
// come from decoding a JSON request body as composite types — a JSON object
// (map[string]interface{}) or array ([]interface{}) — can't be encoded into a
// text/jsonb column by the driver, so they are JSON-marshaled to a string first.
// Everything else (scalars, time.Time, and binary []byte for BYTEA) is left
// untouched.
// encodeParam prepares a Go value for a SQL bind. pgx can't encode arbitrary
// composite Go types (maps, slices, structs from a decoded JSON body or a
// TableOnSubmit hook) into text/jsonb columns, so any composite is JSON-
// marshalled to a string first. Scalars, []byte (BYTEA), and time.Time are
// passed through untouched.
func encodeParam(v interface{}) interface{} {
	switch v.(type) {
	case nil, bool, string, []byte,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64,
		time.Time, *time.Time:
		return v
	}
	switch reflect.TypeOf(v).Kind() {
	case reflect.Map, reflect.Slice, reflect.Array, reflect.Struct:
		if b, err := json.Marshal(v); err == nil {
			return string(b)
		}
	}
	return v
}

func (db *Db) InsertRow(table string, fieldValues map[string]interface{}) (int64, error) {
	fields := []string{}
	values := []interface{}{}
	placeholders := []string{}

	i := 1
	for field, value := range fieldValues {
		fields = append(fields, db.Quote(field))
		values = append(values, encodeParam(value))
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
		log.Printf("SQL error: %s\nError: %v", interpolate(query, values), err)
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
		values = append(values, encodeParam(value))
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
		log.Printf("SQL error: %s\nError: %v", interpolate(query, []interface{}{keyValue}), err)
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
