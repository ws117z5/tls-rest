package module

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"tls-rest/go/lib/db/pgdb"

	"github.com/go-pg/urlstruct"
)

// QueryParams holds pagination and filtering parameters
type QueryParams struct {
	Page    int                    `url:"page,default:1"`
	Limit   int                    `url:"limit,default:20"`
	Sort    string                 `url:"sort"`
	Order   string                 `url:"order,default:asc"`
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
		params.Order = "asc"
	}

	return params, nil
}

// BuildSelectQuery constructs a SELECT query based on fieldset configuration and parameters
func (fe *FieldsetEngine) BuildSelectQuery(params *QueryParams, mode int) (string, []interface{}, error) {
	var selectFields []string
	var whereConditions []string
	var args []interface{}
	argIndex := 1

	v := viewerFromRequest(fe.Request)

	// Build SELECT fields based on mode, withholding access-restricted columns
	// from users who may not read them (system fields are always kept — the
	// client needs id/uuid to route and act).
	for _, field := range fe.Fields {
		if fe.shouldIncludeField(field, mode) && v.fieldReadableInData(field) {
			if field.SQL != "" {
				selectFields = append(selectFields, field.SQL+" AS "+field.Name)
			} else {
				selectFields = append(selectFields, field.Name)
			}
		}
	}

	if len(selectFields) == 0 {
		selectFields = []string{"*"}
	}

	query := fmt.Sprintf("SELECT %s FROM %s", strings.Join(selectFields, ", "), fe.TableName)

	// Build WHERE conditions
	if params.Search != "" && fe.hasSearchableFields() {
		searchConditions := fe.buildSearchConditions(params.Search, &argIndex, &args)
		if len(searchConditions) > 0 {
			whereConditions = append(whereConditions, fmt.Sprintf("(%s)", strings.Join(searchConditions, " OR ")))
		}
	}

	// Build filter conditions
	if params.Filters != nil {
		filterConditions := fe.buildFilterConditions(params.Filters, &argIndex, &args)
		whereConditions = append(whereConditions, filterConditions...)
	}

	// Declared list filters (module <name>/filters.go): turn matching query
	// parameters into WHERE clauses for list mode.
	declaredFilters := fe.buildDeclaredFilterConditions(&argIndex, &args)
	whereConditions = append(whereConditions, declaredFilters...)

	// Row-level access filtering: non-admins only see records whose access level
	// is within their own (admins see everything).
	if !v.isAdmin && fe.hasField("access") {
		whereConditions = append(whereConditions, fmt.Sprintf("access <= $%d", argIndex))
		args = append(args, v.level)
		argIndex++
	}

	// Add WHERE clause if we have conditions
	if len(whereConditions) > 0 {
		query += " WHERE " + strings.Join(whereConditions, " AND ")
	}

	// Add ORDER BY
	if params.Sort != "" && fe.isValidSortField(params.Sort) {
		query += fmt.Sprintf(" ORDER BY %s %s", params.Sort, params.Order)
	} else if fe.hasDefaultSortField() {
		defaultSort := fe.getDefaultSortField()
		query += fmt.Sprintf(" ORDER BY %s %s", defaultSort, params.Order)
	}

	return query, args, nil
}

// BuildCountQuery constructs a COUNT query for pagination
func (fe *FieldsetEngine) BuildCountQuery(params *QueryParams) (string, []interface{}, error) {
	var whereConditions []string
	var args []interface{}
	argIndex := 1

	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", fe.TableName)

	v := viewerFromRequest(fe.Request)

	// Build WHERE conditions (same as select query)
	if params.Search != "" && fe.hasSearchableFields() {
		searchConditions := fe.buildSearchConditions(params.Search, &argIndex, &args)
		if len(searchConditions) > 0 {
			whereConditions = append(whereConditions, fmt.Sprintf("(%s)", strings.Join(searchConditions, " OR ")))
		}
	}

	if params.Filters != nil {
		filterConditions := fe.buildFilterConditions(params.Filters, &argIndex, &args)
		whereConditions = append(whereConditions, filterConditions...)
	}

	// Same declared list filters as the select query, so the total count matches
	// the filtered result set.
	declaredFilters := fe.buildDeclaredFilterConditions(&argIndex, &args)
	whereConditions = append(whereConditions, declaredFilters...)

	// Keep the count consistent with the filtered result set.
	if !v.isAdmin && fe.hasField("access") {
		whereConditions = append(whereConditions, fmt.Sprintf("access <= $%d", argIndex))
		args = append(args, v.level)
		argIndex++
	}

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
		total = pgdb.Coerce[int](countResult["count"])
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

	// Process TYPE_TABLE fields for each result
	for _, result := range results {
		err = fe.ProcessTableFieldsInResult(result)
		if err != nil {
			return nil, fmt.Errorf("failed to process table fields: %w", err)
		}
	}

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
	// Exclude TYPE_TABLE fields from main SELECT - they have their own queries
	if field.Type == TYPE_TABLE {
		return false
	}

	// Include field based on mode (LIST, VIEW, EDIT, etc.)
	// This can be enhanced to check field-specific mode flags
	return !field.Virtual || (mode&MODE_EDIT != 0)
}

func (fe *FieldsetEngine) hasSearchableFields() bool {
	for _, field := range fe.Fields {
		if field.Type == TYPE_STRING || field.Type == TYPE_TEXT || field.Type == TYPE_AUTOCOMPLETE {
			return true
		}
	}
	return false
}

func (fe *FieldsetEngine) buildSearchConditions(search string, argIndex *int, args *[]interface{}) []string {
	var conditions []string

	for _, field := range fe.Fields {
		if field.Type == TYPE_STRING || field.Type == TYPE_TEXT || field.Type == TYPE_AUTOCOMPLETE {
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

func (fe *FieldsetEngine) hasDefaultSortField() bool {
	// Look for an ID field or created field
	for _, field := range fe.Fields {
		if field.Name == "id" || field.Name == "created" || field.Name == "created_at" {
			return true
		}
	}
	return false
}

func (fe *FieldsetEngine) getDefaultSortField() string {
	// Priority: id > created > created_at > first field
	for _, field := range fe.Fields {
		if field.Name == "id" {
			return "id"
		}
	}
	for _, field := range fe.Fields {
		if field.Name == "created" || field.Name == "created_at" {
			return field.Name
		}
	}
	if len(fe.Fields) > 0 {
		return fe.Fields[0].Name
	}
	return "id"
}

func (fe *FieldsetEngine) getFieldByName(name string) *Field {
	for _, field := range fe.Fields {
		if field.Name == name {
			return &field
		}
	}
	return nil
}

// hasField reports whether the module declares a field with the given name.
func (fe *FieldsetEngine) hasField(name string) bool {
	for _, field := range fe.Fields {
		if field.Name == name {
			return true
		}
	}
	return false
}

// FetchTableFieldData fetches data for a TABLE field from database
func (fe *FieldsetEngine) FetchTableFieldData(field Field, parentRecordID interface{}) (interface{}, error) {
	if field.Type != TYPE_TABLE || field.Options == nil {
		return nil, fmt.Errorf("field %s is not a valid table field", field.Name)
	}

	dataSource, ok := field.Options["dataSource"].(string)
	if !ok {
		dataSource = "static"
	}

	switch dataSource {
	case "database":
		return fe.fetchFromDatabaseTable(field, parentRecordID)
	case "query":
		return fe.fetchFromCustomQuery(field, parentRecordID)
	case "static":
		return field.Options["data"], nil
	default:
		return nil, fmt.Errorf("unknown data source: %s", dataSource)
	}
}

// fetchFromDatabaseTable fetches data from a referenced database table
func (fe *FieldsetEngine) fetchFromDatabaseTable(field Field, parentRecordID interface{}) (interface{}, error) {
	db, err := pgdb.GetInstance()
	if err != nil {
		return nil, fmt.Errorf("failed to get database connection: %w", err)
	}

	sourceTable, ok := field.Options["sourceTable"].(string)
	if !ok {
		return nil, fmt.Errorf("sourceTable not configured for field %s", field.Name)
	}

	// Build the query
	var query string
	var args []interface{}

	// Check if there are columns specified
	columns := "*"
	if cols, ok := field.Options["columns"].([]string); ok && len(cols) > 0 {
		columns = strings.Join(cols, ", ")
	}

	// Check if there's a foreign key relationship
	if fk, ok := field.Options["foreignKey"].(map[string]string); ok {
		foreignKeyColumn := fk["column"]
		query = fmt.Sprintf("SELECT %s FROM %s WHERE %s = $1", columns, sourceTable, foreignKeyColumn)
		args = append(args, parentRecordID)
	} else {
		// No foreign key, return all records
		query = fmt.Sprintf("SELECT %s FROM %s", columns, sourceTable)
	}

	// Add any additional query parameters
	if params, ok := field.Options["queryParams"].(map[string]interface{}); ok {
		paramIndex := len(args) + 1
		for column, value := range params {
			if len(args) == 0 {
				query += " WHERE "
			} else {
				query += " AND "
			}
			query += fmt.Sprintf("%s = $%d", column, paramIndex)
			args = append(args, value)
			paramIndex++
		}
	}

	// Execute the query
	results, err := db.RQuery(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch table data: %w", err)
	}

	return results, nil
}

// fetchFromCustomQuery fetches data using a custom SQL query
func (fe *FieldsetEngine) fetchFromCustomQuery(field Field, parentRecordID interface{}) (interface{}, error) {
	db, err := pgdb.GetInstance()
	if err != nil {
		return nil, fmt.Errorf("failed to get database connection: %w", err)
	}

	query, ok := field.Options["query"].(string)
	if !ok {
		return nil, fmt.Errorf("query not configured for field %s", field.Name)
	}

	// Prepare arguments
	var args []interface{}

	// If the query has placeholders, add parent record ID as first parameter
	if strings.Contains(query, "$1") || strings.Contains(query, "?") {
		args = append(args, parentRecordID)
	}

	// Add any additional query parameters
	if params, ok := field.Options["queryParams"].(map[string]interface{}); ok {
		for _, value := range params {
			args = append(args, value)
		}
	}

	// Execute the query
	results, err := db.RQuery(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute custom query: %w", err)
	}

	return results, nil
}

// ProcessTableFieldsInResult processes table fields in query results to fetch their data
func (fe *FieldsetEngine) ProcessTableFieldsInResult(result map[string]interface{}) error {
	for _, field := range fe.Fields {
		if field.Type == TYPE_TABLE {
			// Get the parent record ID for foreign key relationships
			var parentID interface{}
			if id, exists := result["id"]; exists {
				parentID = id
			} else if uuid, exists := result["uuid"]; exists {
				parentID = uuid
			}

			// Fetch table field data
			tableData, err := fe.FetchTableFieldData(field, parentID)
			if err != nil {
				// Log error but don't fail the entire result
				fmt.Printf("Warning: Failed to fetch table field data for %s: %v\n", field.Name, err)
				result[field.Name] = []interface{}{} // Empty array as fallback
			} else {
				result[field.Name] = tableData
			}
		}
	}
	return nil
}
