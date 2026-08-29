package module

import (
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
		Input string `json:"input"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	input := strings.TrimSpace(body.Input)

	var target *Field
	for i := range module.Fields {
		if module.Fields[i].Name == fieldName {
			target = &module.Fields[i]
			break
		}
	}

	options := []string{}
	if target != nil {
		options = resolveAutocomplete(target, input)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"options": options})
}

// resolveAutocomplete runs a field's autocomplete config against the input.
func resolveAutocomplete(f *Field, input string) []string {
	switch f.AutocompleteKind {
	case "function":
		if f.AutocompleteFunc != nil {
			return f.AutocompleteFunc(input)
		}
	case "sql":
		if f.AutocompleteSQL != "" {
			return autocompleteQuery(f.AutocompleteSQL, "%"+input+"%")
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
			return autocompleteQuery(q, pattern)
		}
	}
	return []string{}
}

// autocompleteQuery runs a single-column LIKE query and returns the values.
func autocompleteQuery(query, pattern string) []string {
	db, err := pgdb.GetInstance()
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

	var ctx map[string]interface{}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&ctx)
	}
	if ctx == nil {
		ctx = map[string]interface{}{}
	}

	var target *Field
	for i := range module.Fields {
		if module.Fields[i].Name == fieldName {
			target = &module.Fields[i]
			break
		}
	}
	rows := []map[string]interface{}{}
	if target != nil && target.TableDataFunc != nil {
		if got := target.TableDataFunc(ctx); got != nil {
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
			field.Mode &= mask
			if field.Mode == 0 {
				continue // not granted in any mode -> omit entirely
			}
		}
		visibleFields = append(visibleFields, resolveFieldOptions(field))
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

// resolveFieldOptions populates a select-style field's concrete `options` from
// its dataSource table, so the frontend (which renders a static options list)
// shows choices for table-backed selects. A field is treated as a select when
// its type is a select type or it carries the widget:"select" hint. The source
// table is taken from sourceTable, or from dataSource when that names a table
// (i.e. it isn't one of the reserved static/database/query keywords). The value
// and label columns default to id/name. Non-select fields are returned as-is.
// registeredModuleOptions builds select options {value:id, name:label} from the
// registered modules (menu registry). Called at request time so the registry is
// populated. Falls back to the module id when no label is registered.
// registeredModuleOptions builds select options {value:id, name:label} from the
// registered modules. The list comes from RegisteredModules, which is populated
// unconditionally for every module during Initialize (so it is never empty when
// modules exist) — unlike RegisteredModuleMenu, which is only populated when the
// menu writer is wired. Nicer labels are taken from the menu registry when
// present, falling back to the module id.
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

	ids := make([]string, 0, len(RegisteredModules))
	for id := range RegisteredModules {
		ids = append(ids, id)
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

func resolveFieldOptions(field Field) Field {
	// A field may supply its options via a provider func (WithOptions), resolved
	// here at request time. This handles TYPE_BITMASK_SELECT and any select-style
	// field, and takes priority over static options.
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

	// Lazy source: the set of registered modules (resolved now, at request time,
	// so the registry is fully populated — unlike package-init). Used by the
	// rights modules' "module" select.
	if src, _ := opts["optionsSource"].(string); src == "modules" {
		out := field
		newOpts := make(map[string]interface{}, len(opts)+1)
		for k, v := range opts {
			newOpts[k] = v
		}
		newOpts["options"] = registeredModuleOptions()
		out.Options = newOpts
		return out
	}

	table, _ := opts["sourceTable"].(string)
	if table == "" {
		if ds, ok := opts["dataSource"].(string); ok && ds != "static" && ds != "database" && ds != "query" {
			table = ds
		}
	}
	if table == "" {
		return field // options already provided statically, or nothing to resolve
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
		return field
	}

	options := selectOptionsFromTable(table, valueField, displayField)
	if options == nil {
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

// selectOptionsFromTable reads {name, value} option rows for a select field.
func selectOptionsFromTable(table, valueField, displayField string) []map[string]interface{} {
	db, err := pgdb.GetInstance()
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
			"name":  pgdb.Coerce[string](row["label"]),
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
