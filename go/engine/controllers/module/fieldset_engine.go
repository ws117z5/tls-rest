package module

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"tls-rest/go/engine/controllers/db/pgdb"
	. "tls-rest/go/engine/controllers/field"
	"tls-rest/go/engine/controllers/functions"

	"github.com/go-pg/urlstruct"
)

// QueryParams holds pagination and filtering parameters
type QueryParams struct {
	Page    int                    `url:"page,default:1"`
	Limit   int                    `url:"limit,default:20"`
	Sort    string                 `url:"sort"`
	Order   string                 `url:"order"`
	Search  string                 `url:"search"`
	Filters map[string]interface{} `url:"filters"`
}

// QueryResult contains the result of a query with pagination info
type QueryResult struct {
	Data       interface{} `json:"data"`
	Total      int         `json:"total"`
	Page       int         `json:"page"`
	Limit      int         `json:"limit"`
	TotalPages int         `json:"total_pages"`
}

// FieldsetEngine handles automatic SQL generation and query execution
type FieldsetEngine struct {
	Module    *ModuleAbstract[interface{}]
	TableName string
	Fields    []Field
	Context   context.Context
	Request   *http.Request
}

// NewFieldsetEngine creates a new fieldset engine instance
func NewFieldsetEngine(module *ModuleAbstract[interface{}], tableName string) *FieldsetEngine {
	return &FieldsetEngine{
		Module:    module,
		TableName: tableName,
		Fields:    module.Fields,
	}
}

// WithRequest sets the HTTP request context for parameter parsing
func (fe *FieldsetEngine) WithRequest(r *http.Request) *FieldsetEngine {
	fe.Request = r
	fe.Context = r.Context()
	return fe
}

// ParseQueryParams extracts and validates query parameters from the request
func (fe *FieldsetEngine) ParseQueryParams() (*QueryParams, error) {
	if fe.Request == nil {
		return &QueryParams{Page: 1, Limit: 20, Order: "asc"}, nil
	}

	params := &QueryParams{}
	ctx := fe.Context
	if ctx == nil {
		ctx = context.Background()
	}

	err := urlstruct.Unmarshal(ctx, fe.Request.URL.Query(), params)
	if err != nil {
		return nil, fmt.Errorf("failed to parse query parameters: %w", err)
	}

	// Validate and set defaults
	if params.Page < 1 {
		params.Page = 1
	}
	if params.Limit < 1 || params.Limit > 100 {
		params.Limit = 20
	}
	if params.Order != "asc" && params.Order != "desc" {
		// List defaults to newest-first (DESC); a module may opt into oldest-first
		// (ASC) via ListAscending.
		if fe.Module != nil && fe.Module.ListAscending {
			params.Order = "asc"
		} else {
			params.Order = "desc"
		}
	}

	return params, nil
}

// BuildSelectQuery constructs a SELECT query based on fieldset configuration and parameters
// tableColsCache memoizes each table's real column set (schema is fixed after
// startup module registration, so caching is safe). Failed/empty lookups are
// not cached, so they retry.
var tableColsCache sync.Map // tableName -> map[string]bool (lowercased names)

// tableColumnsCached returns the set of column names (lowercased) that actually
// exist on the table, or an empty map if the table is missing or the lookup
// fails (callers treat empty as "don't filter").
func tableColumnsCached(table string) map[string]bool {
	if v, ok := tableColsCache.Load(table); ok {
		return v.(map[string]bool)
	}
	cols := map[string]bool{}
	if db, err := pgdb.GetInstance(); err == nil {
		if c, err := tableColumns(db, table); err == nil {
			cols = c
		}
	}
	if len(cols) > 0 {
		tableColsCache.Store(table, cols)
	}
	return cols
}

// tableColumns returns the set of column names (lowercased) that currently exist
// on the given table, straight from information_schema.
func tableColumns(db *pgdb.Db, table string) (map[string]bool, error) {
	rows, err := db.RQuery(
		`SELECT column_name FROM information_schema.columns WHERE table_name = $1`, table)
	if err != nil {
		return nil, err
	}
	cols := make(map[string]bool, len(rows))
	for _, r := range rows {
		if c, ok := r["column_name"].(string); ok {
			cols[strings.ToLower(c)] = true
		}
	}
	return cols, nil
}

func (fe *FieldsetEngine) BuildSelectQuery(params *QueryParams, mode int) (string, []interface{}, error) {
	var selectFields []string
	var whereConditions []string
	var args []interface{}
	argIndex := 1

	v := viewerForModule(fe.Request, fe.Module.ID)

	// Build SELECT fields based on mode, withholding access-restricted columns
	// from users who may not read them (system fields are always kept — the
	// client needs id/uuid to route and act).
	// Actual columns of the table, used to skip any fieldset field whose column
	// doesn't exist (fieldset/table drift — e.g. a system field like "updated"
	// that a module omitted but that is still in some fieldset copy). This makes
	// the query resilient instead of failing with "column X does not exist".
	// Empty (table missing / lookup failed) means "don't filter".
	cols := tableColumnsCached(fe.TableName)

	for _, field := range fe.Fields {
		if fe.shouldIncludeField(field, mode) && v.fieldReadableInData(field) {
			if field.SQL != "" {
				selectFields = append(selectFields, field.SQL+" AS "+field.Name)
			} else {
				if len(cols) > 0 && !cols[strings.ToLower(field.Name)] {
					continue // column not in the table; don't select it
				}
				selectFields = append(selectFields, field.Name)
			}
		}
	}

	if len(selectFields) == 0 {
		selectFields = []string{"*"}
	}

	query := fmt.Sprintf("SELECT %s FROM %s", strings.Join(selectFields, ", "), fe.TableName)

	whereConditions = fe.buildScopeConditions(params, v, &argIndex, &args)
	if len(whereConditions) > 0 {
		query += " WHERE " + strings.Join(whereConditions, " AND ")
	}

	// Add ORDER BY
	if params.Sort != "" && fe.isValidSortField(params.Sort) {
		query += fmt.Sprintf(" ORDER BY %s %s", params.Sort, params.Order)
	} else if ds := fe.defaultSortField(); ds != "" {
		query += fmt.Sprintf(" ORDER BY %s %s", ds, params.Order)
	}

	return query, args, nil
}

// buildScopeConditions returns the WHERE conditions common to the list SELECT and
// its COUNT — free-text search, ad-hoc filters, declared list filters, row-level
// access, owner scoping and soft-delete — appending bind values to args and
// advancing argIndex. Sharing this is what keeps the paginated total consistent
// with the rows actually returned.
func (fe *FieldsetEngine) buildScopeConditions(params *QueryParams, v viewer, argIndex *int, args *[]interface{}) []string {
	var conds []string

	// Free-text search across searchable columns.
	if params.Search != "" && fe.hasSearchableFields() {
		if sc := fe.buildSearchConditions(params.Search, argIndex, args); len(sc) > 0 {
			conds = append(conds, fmt.Sprintf("(%s)", strings.Join(sc, " OR ")))
		}
	}

	// Ad-hoc filters[...] parameters.
	if params.Filters != nil {
		conds = append(conds, fe.buildFilterConditions(params.Filters, argIndex, args)...)
	}

	// Declared list filters (module <name>/filters.go).
	conds = append(conds, fe.buildDeclaredFilterConditions(argIndex, args)...)

	// Row-level access: non-admins only see records within their own access level.
	if !v.isAdmin && fe.hasField("access") {
		conds = append(conds, fmt.Sprintf("access <= $%d", *argIndex))
		*args = append(*args, v.level)
		*argIndex++
	}

	// Owner scoping: an OwnerScoped module shows a non-admin only rows they created.
	if fe.Module != nil && fe.Module.OwnerScoped && !v.isAdmin && v.userID > 0 && fe.hasField("created_by") {
		conds = append(conds, fmt.Sprintf("created_by = $%d", *argIndex))
		*args = append(*args, v.userID)
		*argIndex++
	}

	// Soft delete: a SoftDelete module never lists/views deleted rows.
	if fe.Module != nil && fe.Module.SoftDelete && fe.hasField("deleted") {
		conds = append(conds, "deleted IS NOT TRUE")
	}

	return conds
}

// BuildCountQuery constructs a COUNT query for pagination
func (fe *FieldsetEngine) BuildCountQuery(params *QueryParams) (string, []interface{}, error) {
	var args []interface{}
	argIndex := 1

	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", fe.TableName)

	v := viewerFromRequest(fe.Request)
	whereConditions := fe.buildScopeConditions(params, v, &argIndex, &args)
	if len(whereConditions) > 0 {
		query += " WHERE " + strings.Join(whereConditions, " AND ")
	}

	return query, args, nil
}

// ExecuteQuery runs the query and returns results with pagination
func (fe *FieldsetEngine) ExecuteQuery(mode int) (*QueryResult, error) {
	params, err := fe.ParseQueryParams()
	if err != nil {
		return nil, err
	}

	db, err := pgdb.GetInstance()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	// Get total count
	countQuery, countArgs, err := fe.BuildCountQuery(params)
	if err != nil {
		return nil, err
	}

	countResult, err := db.GetOne(countQuery, countArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to get total count: %w", err)
	}

	// GetOne now returns a map directly.
	total := 0
	if countResult != nil {
		total = functions.Coerce[int](countResult["count"])
	}

	// Build and execute main query
	selectQuery, selectArgs, err := fe.BuildSelectQuery(params, mode)
	if err != nil {
		return nil, err
	}

	// Add LIMIT and OFFSET
	offset := (params.Page - 1) * params.Limit
	selectQuery += fmt.Sprintf(" LIMIT %d OFFSET %d", params.Limit, offset)

	// Execute query
	results, err := db.RQuery(selectQuery, selectArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	// TYPE_TABLE fields are excluded from the main SELECT (shouldIncludeField) and
	// loaded on demand via POST /api/modules/{id}/table/{field}; nothing to fold
	// into the row here.

	// Calculate total pages
	totalPages := (total + params.Limit - 1) / params.Limit

	return &QueryResult{
		Data:       results,
		Total:      total,
		Page:       params.Page,
		Limit:      params.Limit,
		TotalPages: totalPages,
	}, nil
}

// Helper methods

func (fe *FieldsetEngine) shouldIncludeField(field Field, mode int) bool {
	// TYPE_TABLE rows load via their own endpoint and are never part of the edit
	// SELECT. A table field WITH an SQL expression is included in list/view only,
	// so a computed display value (e.g. group names resolved from stored ids) can
	// be shown without a separate request.
	if field.Type == TYPE_TABLE {
		return field.SQL != "" && mode&(MODE_LIST|MODE_VIEW) != 0
	}

	// Include field based on mode (LIST, VIEW, EDIT, etc.)
	// This can be enhanced to check field-specific mode flags
	return !field.Virtual || (mode&MODE_EDIT != 0)
}

// searchable reports whether a field participates in the free-text list search:
// a text-like column that has not been opted out with NonSearchable().
func searchable(f Field) bool {
	if !f.Searchable {
		return false
	}
	return f.Type == TYPE_STRING || f.Type == TYPE_TEXT || f.Type == TYPE_AUTOCOMPLETE
}

func (fe *FieldsetEngine) hasSearchableFields() bool {
	for _, field := range fe.Fields {
		if searchable(field) {
			return true
		}
	}
	return false
}

func (fe *FieldsetEngine) buildSearchConditions(search string, argIndex *int, args *[]interface{}) []string {
	var conditions []string

	for _, field := range fe.Fields {
		if searchable(field) {
			conditions = append(conditions, fmt.Sprintf("%s ILIKE $%d", field.Name, *argIndex))
			*args = append(*args, "%"+search+"%")
			*argIndex++
		}
	}

	return conditions
}

func (fe *FieldsetEngine) buildFilterConditions(filters map[string]interface{}, argIndex *int, args *[]interface{}) []string {
	var conditions []string

	for fieldName, value := range filters {
		field := fe.getFieldByName(fieldName)
		if field == nil || !field.Filterable {
			continue
		}

		if field.SQLWhere != "" {
			// Use custom WHERE clause from field
			conditions = append(conditions, fmt.Sprintf(field.SQLWhere, fmt.Sprintf("$%d", *argIndex)))
		} else {
			// Default equality check
			conditions = append(conditions, fmt.Sprintf("%s = $%d", fieldName, *argIndex))
		}

		*args = append(*args, value)
		*argIndex++
	}

	return conditions
}

func (fe *FieldsetEngine) isValidSortField(fieldName string) bool {
	field := fe.getFieldByName(fieldName)
	return field != nil && !field.Virtual
}

// defaultSortField returns the column to ORDER BY when the request names none:
// "id" if present, else a "created"/"created_at" column, else "" (no ORDER BY).
func (fe *FieldsetEngine) defaultSortField() string {
	var created string
	for _, field := range fe.Fields {
		if field.Name == "id" {
			return "id"
		}
		if created == "" && (field.Name == "created" || field.Name == "created_at") {
			created = field.Name
		}
	}
	return created
}

func (fe *FieldsetEngine) getFieldByName(name string) *Field {
	for i := range fe.Fields {
		if fe.Fields[i].Name == name {
			return &fe.Fields[i]
		}
	}
	return nil
}

// hasField reports whether the module declares a field with the given name.
func (fe *FieldsetEngine) hasField(name string) bool {
	return fe.getFieldByName(name) != nil
}
