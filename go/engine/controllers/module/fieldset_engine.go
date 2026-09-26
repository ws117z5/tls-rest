package module

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	constants "tls-rest/go/app/constants"
	"tls-rest/go/engine/controllers/db/pgdb"
	. "tls-rest/go/engine/controllers/field"
	"tls-rest/go/engine/controllers/functions"
	"tls-rest/go/engine/controllers/request"

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

// WithRequest returns a per-request copy bound to r; fe itself is shared by concurrent requests and must never be mutated.
func (fe *FieldsetEngine) WithRequest(r *http.Request) *FieldsetEngine {
	c := *fe
	c.Request = r
	c.Context = r.Context()
	return &c
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

	if params.Page < 1 {
		params.Page = 1
	}
	if params.Limit < 1 || params.Limit > 100 {
		params.Limit = 20
	}
	if params.Order != "asc" && params.Order != "desc" {
		if fe.Module != nil && fe.Module.ListAscending {
			params.Order = "asc"
		} else {
			params.Order = "desc"
		}
	}

	return params, nil
}

// tableColsCache memoizes each table's real column set; failed/empty lookups
// aren't cached, so they retry.
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

// UnmarshalURL satisfies the urlstruct interface
func (p *QueryParams) UnmarshalURL(values url.Values) error {
	if p.Filters == nil {
		p.Filters = make(map[string]interface{})
	}

	for key, vals := range values {
		if len(vals) == 0 || vals[0] == "" {
			continue
		}

		if strings.HasPrefix(key, "filters.") {
			filterKey := strings.TrimPrefix(key, "filters.")
			p.Filters[filterKey] = vals[0]
		}
	}

	return nil
}

// totalCountAlias is the COUNT(*) OVER() column ExecuteQuery rides along
// with the page's rows, prefixed to avoid colliding with a real column.
const totalCountAlias = "__fieldset_total"

// BuildSelectQuery constructs a SELECT for mode, withholding fields the
// viewer can't read and any fieldset field whose column no longer exists on
// the table (fieldset/table drift).
func (fe *FieldsetEngine) BuildSelectQuery(params *QueryParams, mode int) (string, []interface{}, error) {
	return fe.buildSelectQuery(params, mode, false)
}

// buildSelectQuery is BuildSelectQuery's implementation; includeTotal
// prepends a COUNT(*) OVER() column so ExecuteQuery gets the paginated total
// from the same round trip instead of a separate COUNT query.
func (fe *FieldsetEngine) buildSelectQuery(params *QueryParams, mode int, includeTotal bool) (string, []interface{}, error) {
	var selectFields []string
	var args []interface{}
	argIndex := 1

	v := viewerForModule(fe.Request, fe.Module.ID)
	cols := tableColumnsCached(fe.TableName)

	for _, field := range fe.Fields {
		if fe.shouldIncludeField(field, mode) && v.fieldReadableInData(field) {
			if field.SQL != "" {
				selectFields = append(selectFields, field.SQL+" AS "+field.Name)
			} else {
				if len(cols) > 0 && !cols[strings.ToLower(field.Name)] {
					continue
				}
				selectFields = append(selectFields, field.Name)
			}
		}
	}

	if len(selectFields) == 0 {
		selectFields = []string{"*"}
	}

	if includeTotal {
		selectFields = append([]string{"COUNT(*) OVER() AS " + totalCountAlias}, selectFields...)
	}

	query := fmt.Sprintf("SELECT %s FROM %s", strings.Join(selectFields, ", "), fe.TableName)

	whereConditions := fe.buildScopeConditions(params, v, &argIndex, &args)
	if len(whereConditions) > 0 {
		query += " WHERE " + strings.Join(whereConditions, " AND ")
	}

	if params.Sort != "" && fe.isValidSortField(v, params.Sort) {
		query += fmt.Sprintf(" ORDER BY %s %s", params.Sort, params.Order)
	} else if ds := fe.defaultSortField(); ds != "" {
		query += fmt.Sprintf(" ORDER BY %s %s", ds, params.Order)
	}

	return query, args, nil
}

// buildScopeConditions returns the WHERE conditions shared by the list SELECT
// and its COUNT — key addressing, search, filters, row-level access, owner
// scoping and soft-delete — so the paginated total matches the rows returned.
func (fe *FieldsetEngine) buildScopeConditions(params *QueryParams, v viewer, argIndex *int, args *[]interface{}) []string {
	var conds []string

	if fe.Request != nil {
		key := fe.keyField()
		if kv := request.From(fe.Request).String(key); kv != "" {
			var bind interface{} = kv
			if key == "id" {
				if n, err := strconv.ParseInt(kv, 10, 64); err == nil {
					bind = n
				}
			}
			conds = append(conds, fmt.Sprintf("%s = $%d", key, *argIndex))
			*args = append(*args, bind)
			*argIndex++
		}
	}

	if params.Search != "" && fe.hasSearchableFields(v) {
		if sc := fe.buildSearchConditions(v, params.Search, argIndex, args); len(sc) > 0 {
			conds = append(conds, fmt.Sprintf("(%s)", strings.Join(sc, " OR ")))
		}
	}

	if params.Filters != nil {
		conds = append(conds, fe.buildFilterConditions(params.Filters, argIndex, args)...)
	}

	conds = append(conds, fe.buildDeclaredFilterConditions(argIndex, args)...)

	if cond := fe.buildVisibilityCondition(v, argIndex, args); cond != "" {
		conds = append(conds, cond)
	}

	if fe.Module != nil && fe.Module.OwnerScoped && !v.isAdmin && v.userID > 0 && fe.hasField("created_by") {
		conds = append(conds, fmt.Sprintf("created_by = $%d", *argIndex))
		*args = append(*args, v.userID)
		*argIndex++
	}

	if fe.Module != nil && fe.Module.SoftDelete && fe.hasField("deleted") {
		conds = append(conds, "deleted IS NOT TRUE")
	}

	return conds
}

// buildVisibilityCondition returns the row-level visibility condition for a
// non-admin viewer (empty for admins): the ACL sharing-list model if
// VisibilityUsersField/VisibilityGroupsField is set, else the access-level gate.
func (fe *FieldsetEngine) buildVisibilityCondition(v viewer, argIndex *int, args *[]interface{}) string {
	if v.isAdmin || !fe.hasField("access") {
		return ""
	}

	aclMode := fe.Module != nil && (fe.Module.VisibilityUsersField != "" || fe.Module.VisibilityGroupsField != "")

	if aclMode {
		var alt []string

		if v.userID > 0 && fe.hasField("created_by") {
			alt = append(alt, fmt.Sprintf("created_by = $%d", *argIndex))
			*args = append(*args, v.userID)
			*argIndex++
		}
		if f := fe.Module.VisibilityUsersField; f != "" && v.userID > 0 && fe.hasField(f) {
			alt = append(alt, fmt.Sprintf(
				"EXISTS (SELECT 1 FROM jsonb_array_elements_text(%s) AS vu WHERE vu::int = $%d)",
				f, *argIndex,
			))
			*args = append(*args, v.userID)
			*argIndex++
		}
		if f := fe.Module.VisibilityGroupsField; f != "" && fe.hasField(f) {
			if v.userID > 0 {
				alt = append(alt, fmt.Sprintf(
					`EXISTS (SELECT 1 FROM jsonb_array_elements_text(%s) AS vg `+
						`WHERE vg::int IN (SELECT jsonb_array_elements_text(u.groups)::int FROM users u WHERE u.id = $%d))`,
					f, *argIndex,
				))
				*args = append(*args, v.userID)
				*argIndex++
			} else {
				alt = append(alt, fmt.Sprintf(
					"EXISTS (SELECT 1 FROM jsonb_array_elements_text(%s) AS vg WHERE vg::int = $%d)",
					f, *argIndex,
				))
				*args = append(*args, constants.GuestGroupID)
				*argIndex++
			}
		}

		if len(alt) == 0 {
			return "FALSE"
		}
		if len(alt) == 1 {
			return alt[0]
		}
		return "(" + strings.Join(alt, " OR ") + ")"
	}

	alt := []string{fmt.Sprintf("access <= $%d", *argIndex)}
	*args = append(*args, v.level)
	*argIndex++

	if fe.Module != nil {
		if f := fe.Module.VisibilityField; f != "" && fe.hasField(f) {
			alt = append(alt, fmt.Sprintf("%s = true", f))
			if v.userID > 0 && fe.hasField("created_by") {
				alt = append(alt, fmt.Sprintf("created_by = $%d", *argIndex))
				*args = append(*args, v.userID)
				*argIndex++
			}
		}
	}

	if len(alt) == 1 {
		return alt[0]
	}
	return "(" + strings.Join(alt, " OR ") + ")"
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

// ExecuteQuery runs the page's SELECT and its total count as one round trip
// (COUNT(*) OVER() riding on the row query) instead of two.
func (fe *FieldsetEngine) ExecuteQuery(mode int) (*QueryResult, error) {
	params, err := fe.ParseQueryParams()
	if err != nil {
		return nil, err
	}

	ctx := fe.Context
	if ctx == nil {
		ctx = context.Background()
	}
	db, err := pgdb.GetInstanceCtx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	selectQuery, selectArgs, err := fe.buildSelectQuery(params, mode, true)
	if err != nil {
		return nil, err
	}

	offset := (params.Page - 1) * params.Limit
	selectQuery += fmt.Sprintf(" LIMIT %d OFFSET %d", params.Limit, offset)

	results, err := db.RQuery(selectQuery, selectArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	total, err := fe.totalFromResults(db, results, params)
	if err != nil {
		return nil, err
	}

	totalPages := (total + params.Limit - 1) / params.Limit

	return &QueryResult{
		Data:       results,
		Total:      total,
		Page:       params.Page,
		Limit:      params.Limit,
		TotalPages: totalPages,
	}, nil
}

// totalFromResults reads totalCountAlias off the first row and strips it from
// every row. An empty page (filter matched nothing, or params.Page overshot
// the last page) leaves no row to read it from, so that case alone falls back
// to a real COUNT query.
func (fe *FieldsetEngine) totalFromResults(db *pgdb.Db, results []map[string]interface{}, params *QueryParams) (int, error) {
	if len(results) > 0 {
		total := functions.Coerce[int](results[0][totalCountAlias])
		for _, row := range results {
			delete(row, totalCountAlias)
		}
		return total, nil
	}

	countQuery, countArgs, err := fe.BuildCountQuery(params)
	if err != nil {
		return 0, err
	}
	countResult, err := db.GetOne(countQuery, countArgs...)
	if err != nil {
		return 0, fmt.Errorf("failed to get total count: %w", err)
	}
	if countResult == nil {
		return 0, nil
	}
	return functions.Coerce[int](countResult["count"]), nil
}

// shouldIncludeField reports whether field belongs in the SELECT for mode: a
// TYPE_TABLE field only when it has SQL (a computed display value) and mode
// is list/view; a virtual field (also computed via SQL, e.g. posts.author or
// papers.has_password) the same — it has no real column to write, so it's
// never meaningful in create/edit.
func (fe *FieldsetEngine) shouldIncludeField(field Field, mode int) bool {
	if field.Type == TYPE_TABLE || field.Virtual {
		return field.SQL != "" && mode&(MODE_LIST|MODE_VIEW) != 0
	}
	return true
}

// searchable reports whether a field participates in the free-text list search:
// a text-like column that has not been opted out with NonSearchable().
func searchable(f Field) bool {
	if !f.Searchable {
		return false
	}
	return f.Type == TYPE_STRING || f.Type == TYPE_TEXT
}

// hasSearchableFields reports whether any field the viewer may read takes part in search.
func (fe *FieldsetEngine) hasSearchableFields(v viewer) bool {
	for _, field := range fe.Fields {
		if searchable(field) && v.fieldReadableInData(field) {
			return true
		}
	}
	return false
}

func (fe *FieldsetEngine) buildSearchConditions(v viewer, search string, argIndex *int, args *[]interface{}) []string {
	var conditions []string

	for _, field := range fe.Fields {
		if searchable(field) && v.fieldReadableInData(field) {
			conditions = append(conditions, fmt.Sprintf("%s ILIKE $%d", field.Name, *argIndex))
			*args = append(*args, "%"+search+"%")
			*argIndex++
		}
	}

	return conditions
}

func (fe *FieldsetEngine) buildFilterConditions(filters map[string]interface{}, argIndex *int, args *[]interface{}) []string {
	var conditions []string

	// Same gate as declared filters: needs MODE_FILTERS and a field the viewer may see and filter by.
	gated := fe.Module != nil && fe.Request != nil
	var v viewer
	if gated {
		v = viewerForModule(fe.Request, fe.Module.ID)
		if !v.canFilter(fe.Module.ID) {
			return nil
		}
	}

	for fieldName, value := range filters {
		field := fe.getFieldByName(fieldName)
		if field == nil || !field.Filterable {
			continue
		}
		if gated && !(v.fieldVisibleInSchema(*field) && v.canFilterField(fieldName)) {
			continue
		}

		if field.SQLWhere != "" {
			conditions = append(conditions, fmt.Sprintf(field.SQLWhere, fmt.Sprintf("$%d", *argIndex)))
		} else {
			conditions = append(conditions, fmt.Sprintf("%s = $%d", fieldName, *argIndex))
		}

		*args = append(*args, value)
		*argIndex++
	}

	return conditions
}

// isValidSortField allows sorting only by stored columns the viewer may read; otherwise ordering leaks hidden values.
func (fe *FieldsetEngine) isValidSortField(v viewer, fieldName string) bool {
	field := fe.getFieldByName(fieldName)
	return field != nil && !field.Virtual && v.fieldReadableInData(*field)
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

// keyField is the column that addresses a single record (module KeyField, else
// "id") — mirrors BaseController.keyField.
func (fe *FieldsetEngine) keyField() string {
	if fe.Module != nil && fe.Module.KeyField != "" {
		return fe.Module.KeyField
	}
	return "id"
}
