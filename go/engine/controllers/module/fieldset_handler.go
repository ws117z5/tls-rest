package module

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"

	"strings"
	"tls-rest/go/engine/controllers/db/cache"
	"tls-rest/go/engine/controllers/db/pgdb"
	. "tls-rest/go/engine/controllers/field"
	"tls-rest/go/engine/controllers/functions"

	"github.com/gorilla/mux"
)

// FieldsetHandler handles fieldset API requests
type FieldsetHandler struct {
	modules map[string]*ModuleAbstract[interface{}]
}

// NewFieldsetHandler creates a new fieldset handler
func NewFieldsetHandler() *FieldsetHandler {
	return &FieldsetHandler{
		modules: make(map[string]*ModuleAbstract[interface{}]),
	}
}

// RegisterModule registers a module for fieldset API and ensures table exists
func (fh *FieldsetHandler) RegisterModule(module *ModuleAbstract[interface{}]) {
	fh.modules[module.ID] = module

	// Try to ensure the table exists for this module, but don't panic if it fails
	// This is deferred to avoid blocking module initialization
	go func() {
		defer func() {
			if r := recover(); r != nil {
				println("Warning: Panic during table creation for module", module.ID, ":", fmt.Sprintf("%v", r))
			}
		}()

		err := module.EnsureTableExists()
		if err != nil {
			// Log the error but don't fail the registration
			// This allows the application to continue running even if table creation fails
			println("Warning: Failed to ensure table exists for module", module.ID, ":", err.Error())
		}
	}()
}

// GetFieldset handles POST /api/modules/{moduleId}/fieldset. It returns the full
// authority-visible fieldset (all modes); the client filters by mode. mode is no
// longer a parameter, so the fieldset is cached once per module.
// GetAutocomplete handles POST /api/modules/{moduleId}/autocomplete/{field}.
// Body: {"input": "..."}. Resolves the field's autocomplete config (function,
// sql, or source) and returns {"options": ["..."]}.
func (fh *FieldsetHandler) GetAutocomplete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	moduleId := vars["moduleId"]
	fieldName := vars["field"]
	if moduleId == "" || fieldName == "" {
		http.Error(w, "module and field are required", http.StatusBadRequest)
		return
	}
	module, exists := fh.modules[moduleId]
	if !exists {
		http.Error(w, "Module not found", http.StatusNotFound)
		return
	}

	var body struct {
		Input  string                 `json:"input"`
		Values map[string]interface{} `json:"values"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	input := strings.TrimSpace(body.Input)

	target := findField(module.Fields, fieldName)

	options := []AutoOption{}
	if target != nil {
		options = resolveAutocomplete(r.Context(), target, input, body.Values)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"options": options})
}

// resolveAutocomplete runs a field's autocomplete config against the input,
// returning value/label options. `values` carries the sibling field values of
// the record being edited, so a function-kind autocomplete can branch on them.
func resolveAutocomplete(ctx context.Context, f *Field, input string, values map[string]interface{}) []AutoOption {
	switch f.AutocompleteKind {
	case "function":
		if f.AutocompleteFunc != nil {
			return f.AutocompleteFunc(ctx, input, values)
		}
	case "sql":
		if f.AutocompleteSQL != "" {
			return stringsToOptions(autocompleteQuery(ctx, f.AutocompleteSQL, "%"+input+"%"))
		}
	case "source":
		if len(f.AutocompleteSource) >= 2 && validIdent(f.AutocompleteSource[0]) && validIdent(f.AutocompleteSource[1]) {
			table, col := f.AutocompleteSource[0], f.AutocompleteSource[1]
			match := "full"
			if len(f.AutocompleteSource) >= 3 {
				match = f.AutocompleteSource[2]
			}
			pattern := input + "%"
			switch match {
			case "right":
				pattern = "%" + input
			case "full":
				pattern = "%" + input + "%"
			}
			q := "SELECT DISTINCT " + col + " FROM " + table + " WHERE " + col + " LIKE $1 ORDER BY " + col + " LIMIT 20"
			return stringsToOptions(autocompleteQuery(ctx, q, pattern))
		}
	}
	return []AutoOption{}
}

// findField returns a pointer to the named field within fields, or nil when the
// fieldset has no such field.
func findField(fields []Field, name string) *Field {
	for i := range fields {
		if fields[i].Name == name {
			return &fields[i]
		}
	}
	return nil
}

// stringsToOptions maps plain suggestion strings to value==label options.
func stringsToOptions(ss []string) []AutoOption {
	out := make([]AutoOption, 0, len(ss))
	for _, s := range ss {
		out = append(out, AutoOption{Value: s, Label: s})
	}
	return out
}

// autocompleteQuery runs a single-column LIKE query and returns the values.
func autocompleteQuery(ctx context.Context, query, pattern string) []string {
	db, err := pgdb.GetInstanceCtx(ctx)
	if err != nil {
		return []string{}
	}
	rows, err := db.RQuery(query, pattern)
	if err != nil {
		return []string{}
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		for _, v := range r { // single column
			if s, ok := v.(string); ok && s != "" {
				out = append(out, s)
			}
			break
		}
	}
	return out
}

// GetTableData handles POST /api/modules/{moduleId}/table/{field}. It runs the
// named TYPE_TABLE field's TableData hook with the posted record values as
// context (e.g. the sibling "module" select) and returns the resulting rows.
func (fh *FieldsetHandler) GetTableData(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	moduleId := vars["moduleId"]
	fieldName := vars["field"]
	if moduleId == "" || fieldName == "" {
		http.Error(w, "module and field are required", http.StatusBadRequest)
		return
	}
	module, exists := fh.modules[moduleId]
	if !exists {
		http.Error(w, "Module not found", http.StatusNotFound)
		return
	}

	var data map[string]interface{}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&data)
	}
	if data == nil {
		data = map[string]interface{}{}
	}

	target := findField(module.Fields, fieldName)
	rows := []map[string]interface{}{}
	if target != nil && target.TableDataFunc != nil {
		if got := target.TableDataFunc(r.Context(), data); got != nil {
			rows = got
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"rows": rows})
}

type FieldsetPayload struct {
	Hash string `json:"hash"`
}

func (fh *FieldsetHandler) GetFieldset(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	moduleId := vars["moduleId"]

	if moduleId == "" {
		http.Error(w, "Module ID is required", http.StatusBadRequest)
		return
	}

	module, exists := fh.modules[moduleId]
	if !exists {
		http.Error(w, "Module not found", http.StatusNotFound)
		return
	}

	// The client may send the hashsum it already has; we answer 304 only when it
	// matches the FRESHLY-computed hash below (not a stale session-stored one),
	// so a rebuild that changes the fieldset always invalidates old client caches.

	clientHash, _ := functions.PostParam[string]("hash", r)

	//clientHash := r.FormValue("hash")
	session := cache.SessionFromContext(ctx)

	// The full fieldset is returned (every field the viewer may see); the client
	// filters by mode using each field's own Mode bitmask. That means one cached
	// fieldset per module (not per module+mode). System fields are admin-only;
	// access-gated fields are hidden from lower levels.
	v := viewerForModule(r, module.ID)
	var visibleFields []Field
	for _, field := range module.Fields {
		if !v.fieldVisibleInSchema(field) {
			continue
		}
		// Per-field rights: restrict the field to only its granted modes, so it
		// appears in (say) view but not edit. mask -1 means "no restriction".
		if mask := v.fieldModeMask(field.Name); mask != -1 {
			field.Mode &= mask | MODE_LOG | MODE_MULTIPLEUPDATE | MODE_SUBMIT
			if field.Mode == 0 {
				continue // not granted in any mode -> omit entirely
			}
		}
		visibleFields = append(visibleFields, field)
	}

	// Every table-backed select's options come from one combined query (see
	// fetchTableFieldOptions) instead of a separate round trip per field.
	tableOpts := fetchTableFieldOptions(ctx, collectTableFieldRefs(visibleFields, "f"))
	for i, field := range visibleFields {
		visibleFields[i] = resolveFieldOptions(ctx, field, v, fmt.Sprintf("f%d", i), tableOpts)
	}

	// Hashsum of the authority-scoped fieldset. Returned to the client (stored in
	// localStorage) and remembered on the session so the fast path can
	// short-circuit subsequent requests.
	hash := hashFieldset(visibleFields)

	// 304 only when the client already has this exact (current) fieldset.
	if clientHash != "" && clientHash == hash {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	if session != nil {
		if session.Fieldset == nil {
			session.Fieldset = map[string]string{}
		}
		if session.Fieldset[moduleId] != hash {
			session.Fieldset[moduleId] = hash
			// Persist the mutated session (SessionFromContext returns a copy).
			// The session key is the X-Session-ID cookie value (see ManageSession).
			if c, err := r.Cookie("X-Session-ID"); err == nil && c.Value != "" {
				cache.SessionCacheInstance.Set(c.Value, *session)
			}
		}
	}

	// Prepare response
	response := map[string]interface{}{
		"id":     module.ID,
		"name":   module.Name,
		"fields": visibleFields,
		"hash":   hash,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// hashFieldset returns a stable hex SHA-256 of the visible fields. Go's
// json.Marshal sorts map keys, so the encoding (and thus the hash) is
// deterministic for a given fieldset and viewer authority.
func hashFieldset(fields []Field) string {
	b, err := json.Marshal(fields)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// GetModules handles GET /api/modules
func (fh *FieldsetHandler) GetModules(w http.ResponseWriter, r *http.Request) {
	modules := make([]map[string]interface{}, 0, len(fh.modules))

	for _, module := range fh.modules {
		modules = append(modules, map[string]interface{}{
			"id":   module.ID,
			"name": module.Name,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"modules": modules,
	})
}

// registeredModuleOptions builds select options {value:id, name:label} from
// RegisteredModules, labeled via the menu registry when present.
func registeredModuleOptions() []map[string]interface{} {
	labels := map[string]string{}
	for _, m := range RegisteredModuleMenu() {
		name := m.Description
		if name == "" {
			name = m.Name
		}
		if name != "" {
			labels[m.ID] = name
		}
	}
	for _, p := range RegisteredPageMenu() {
		if p.Name != "" {
			labels[p.ID] = p.Name
		}
	}

	ids := make([]string, 0, len(RegisteredModules)+len(RegisteredPageMenu()))
	for id := range RegisteredModules {
		ids = append(ids, id)
	}
	for _, p := range RegisteredPageMenu() {
		ids = append(ids, p.ID)
	}
	sort.Strings(ids)

	opts := make([]map[string]interface{}, 0, len(ids))
	for _, id := range ids {
		name := labels[id]
		if name == "" {
			name = id
		}
		opts = append(opts, map[string]interface{}{"value": id, "name": name})
	}
	return opts
}

// resolveFieldOptions fills in a field's concrete `options` list at request time
// (from a WithOptions provider, the registered-modules source, or a table-backed
// select, the latter pre-fetched in tableOpts by fetchTableFieldOptions) so the
// client can render it. For a TYPE_TABLE field it recurses into the column
// fieldset, resolving each column the same way. key must match the one
// collectTableFieldRefs generated for this same field. v is the requesting
// viewer, so an option source can scope its choices to the user's authority.
func resolveFieldOptions(ctx context.Context, field Field, v viewer, key string, tableOpts map[string][]map[string]interface{}) Field {
	// TYPE_TABLE: the table field carries no options list of its own; resolve
	// each column so select columns arrive with their choices populated.
	if field.Type == TYPE_TABLE && len(field.TableColumns) > 0 {
		cols := make([]Field, len(field.TableColumns))
		for i, c := range field.TableColumns {
			cols[i] = resolveFieldOptions(ctx, c, v, fmt.Sprintf("%s_c%d", key, i), tableOpts)
		}
		field.TableColumns = cols
		return field
	}

	// A field may supply its options via a provider func, resolved here at request
	// time (takes priority over a static list). WithOptionsCtx additionally gets
	// the requesting user's authority so the module — not the engine — decides
	// which choices that user may see.
	if field.OptionsCtxFunc != nil {
		out := field
		newOpts := make(map[string]interface{}, len(field.Options)+1)
		for k, val := range field.Options {
			newOpts[k] = val
		}
		newOpts["options"] = field.OptionsCtxFunc(ctx, map[string]interface{}{
			"userID":  v.userID,
			"isAdmin": v.isAdmin,
			"level":   v.level,
		})
		out.Options = newOpts
		return out
	}
	if field.OptionsFunc != nil {
		out := field
		newOpts := make(map[string]interface{}, len(field.Options)+1)
		for k, v := range field.Options {
			newOpts[k] = v
		}
		newOpts["options"] = field.OptionsFunc()
		out.Options = newOpts
		return out
	}

	opts := field.Options
	if opts == nil {
		return field
	}

	widget, _ := opts["widget"].(string)
	if field.Type != TYPE_SELECT && field.Type != TYPE_SELECT_ADDNEW && widget != "select" {
		return field
	}

	// Lazy source: the set of registered modules (registry is fully populated
	// now, unlike at package-init). Used by the rights modules' "module" select.
	if src, _ := opts["optionsSource"].(string); src == "modules" {
		out := field
		newOpts := make(map[string]interface{}, len(opts)+1)
		for k, val := range opts {
			newOpts[k] = val
		}
		newOpts["options"] = registeredModuleOptions()
		out.Options = newOpts
		return out
	}

	// Table-backed select: options were already fetched for every field in one
	// combined query (fetchTableFieldOptions); absent here means the field had
	// no table configured, or table/column identifiers didn't validate.
	options, ok := tableOpts[key]
	if !ok {
		return field
	}

	// Copy the options map so we never mutate the shared module definition.
	newOpts := make(map[string]interface{}, len(opts)+1)
	for k, val := range opts {
		newOpts[k] = val
	}
	newOpts["options"] = options
	field.Options = newOpts
	return field
}

// tableFieldRef is a table-backed select awaiting resolution, keyed to match
// the field tree position resolveFieldOptions will later look it up with.
type tableFieldRef struct {
	key          string
	table        string
	valueField   string
	displayField string
}

// collectTableFieldRefs walks fields (recursing into TYPE_TABLE columns) and
// returns every table-backed select field, keyed the same way
// resolveFieldOptions derives keys for its own field tree walk.
func collectTableFieldRefs(fields []Field, prefix string) []tableFieldRef {
	var refs []tableFieldRef
	for i, field := range fields {
		key := fmt.Sprintf("%s%d", prefix, i)
		if field.Type == TYPE_TABLE && len(field.TableColumns) > 0 {
			refs = append(refs, collectTableFieldRefs(field.TableColumns, key+"_c")...)
			continue
		}
		if ref, ok := tableFieldRefFor(field, key); ok {
			refs = append(refs, ref)
		}
	}
	return refs
}

// tableFieldRefFor reports whether field is a table-backed select with valid
// identifiers, applying the same rules resolveFieldOptions's table branch used to.
func tableFieldRefFor(field Field, key string) (tableFieldRef, bool) {
	if field.OptionsCtxFunc != nil || field.OptionsFunc != nil {
		return tableFieldRef{}, false
	}
	opts := field.Options
	if opts == nil {
		return tableFieldRef{}, false
	}
	widget, _ := opts["widget"].(string)
	if field.Type != TYPE_SELECT && field.Type != TYPE_SELECT_ADDNEW && widget != "select" {
		return tableFieldRef{}, false
	}
	if src, _ := opts["optionsSource"].(string); src == "modules" {
		return tableFieldRef{}, false
	}

	table, _ := opts["sourceTable"].(string)
	if table == "" {
		if ds, ok := opts["dataSource"].(string); ok && ds != "static" && ds != "database" && ds != "query" {
			table = ds
		}
	}
	if table == "" {
		return tableFieldRef{}, false
	}

	valueField, _ := opts["valueField"].(string)
	if valueField == "" {
		valueField = "id"
	}
	displayField, _ := opts["displayField"].(string)
	if displayField == "" {
		displayField = "name"
	}
	if !validIdent(table) || !validIdent(valueField) || !validIdent(displayField) {
		return tableFieldRef{}, false
	}
	return tableFieldRef{key: key, table: table, valueField: valueField, displayField: displayField}, true
}

//https://data.statmt.org/opus-100-corpus/v1.0/supervised/en-ru/opus.en-ru-train.en
//https://data.statmt.org/opus-100-corpus/v1.0/supervised/en-ru/opus.en-ru-train.ru

// fetchTableFieldOptions resolves every ref's {value, label} options in one
// UNION ALL query — one round trip for a whole fieldset instead of one query
// per table-backed select field. Falls back to querying each ref on its own
// (fetchTableFieldOptionsIndividually) only if the combined query itself
// fails, so one stale table/column doesn't cost every other field its options.
func fetchTableFieldOptions(ctx context.Context, refs []tableFieldRef) map[string][]map[string]interface{} {
	if len(refs) == 0 {
		return nil
	}
	db, err := pgdb.GetInstanceCtx(ctx)
	if err != nil {
		return nil
	}

	branches := make([]string, len(refs))
	for i, ref := range refs {
		branches[i] = fmt.Sprintf(
			"SELECT '%s' AS __field, %s::text AS value, %s AS label FROM %s",
			ref.key, db.Quote(ref.valueField), db.Quote(ref.displayField), db.Quote(ref.table),
		)
	}
	query := "SELECT * FROM (" + strings.Join(branches, " UNION ALL ") + ") t ORDER BY __field, label"

	rows, err := db.GetAll(query)
	if err != nil {
		return fetchTableFieldOptionsIndividually(ctx, refs)
	}

	out := make(map[string][]map[string]interface{}, len(refs))
	for _, ref := range refs {
		out[ref.key] = []map[string]interface{}{}
	}
	for _, row := range rows {
		key := functions.Coerce[string](row["__field"])
		out[key] = append(out[key], map[string]interface{}{
			"value": row["value"],
			"name":  functions.Coerce[string](row["label"]),
		})
	}
	return out
}

// fetchTableFieldOptionsIndividually is fetchTableFieldOptions's per-ref
// fallback: same {value, label} shape, one query per ref.
func fetchTableFieldOptionsIndividually(ctx context.Context, refs []tableFieldRef) map[string][]map[string]interface{} {
	out := make(map[string][]map[string]interface{}, len(refs))
	for _, ref := range refs {
		if options := selectOptionsFromTable(ctx, ref.table, ref.valueField, ref.displayField); options != nil {
			out[ref.key] = options
		}
	}
	return out
}

// selectOptionsFromTable reads {name, value} option rows for a single
// table-backed select field — used only by fetchTableFieldOptionsIndividually's
// per-ref fallback when the combined query fails.
func selectOptionsFromTable(ctx context.Context, table, valueField, displayField string) []map[string]interface{} {
	db, err := pgdb.GetInstanceCtx(ctx)
	if err != nil {
		return nil
	}
	query := fmt.Sprintf("SELECT %s AS value, %s AS label FROM %s ORDER BY %s",
		db.Quote(valueField), db.Quote(displayField), db.Quote(table), db.Quote(displayField))
	rows, err := db.GetAll(query)
	if err != nil {
		return nil
	}
	options := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		options = append(options, map[string]interface{}{
			"value": row["value"],
			"name":  functions.Coerce[string](row["label"]),
		})
	}
	return options
}

// validIdent guards a value used as a SQL identifier (table/column name).
func validIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, c := range s {
		switch {
		case c == '_':
		case c >= 'a' && c <= 'z':
		case c >= 'A' && c <= 'Z':
		case i > 0 && c >= '0' && c <= '9':
		default:
			return false
		}
	}
	return true
}

// Global fieldset handler instance
var GlobalFieldsetHandler = NewFieldsetHandler()

// GetRegisteredModules returns a copy of registered modules for testing
func (fh *FieldsetHandler) GetRegisteredModules() map[string]*ModuleAbstract[interface{}] {
	modulesCopy := make(map[string]*ModuleAbstract[interface{}])
	for k, v := range fh.modules {
		modulesCopy[k] = v
	}
	return modulesCopy
}
