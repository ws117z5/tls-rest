package module

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"

	"tls-rest/go/engine/controllers/db/cache"
	"tls-rest/go/engine/controllers/db/pgdb"
	"tls-rest/go/engine/controllers/field"

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
func (fh *FieldsetHandler) GetFieldset(w http.ResponseWriter, r *http.Request) {
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

	// Fast path (see Session.Fieldset): if the client sends the hashsum it already
	// has and it matches what we last served this session for this module, the
	// fieldset is unchanged — answer 304 without recomputing or re-sending it.
	clientHash := r.URL.Query().Get("hash")
	session := cache.SessionFromContext(r.Context())
	if clientHash != "" && session != nil && session.Fieldset != nil &&
		session.Fieldset[moduleId] == clientHash {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	// The full fieldset is returned (every field the viewer may see); the client
	// filters by mode using each field's own Mode bitmask. That means one cached
	// fieldset per module (not per module+mode). System fields are admin-only;
	// access-gated fields are hidden from lower levels.
	v := viewerFromRequest(r)
	var visibleFields []field.Field
	for _, field := range module.Fields {
		if v.fieldVisibleInSchema(field) {
			visibleFields = append(visibleFields, resolveFieldOptions(field))
		}
	}

	// Hashsum of the authority-scoped fieldset. Returned to the client (stored in
	// localStorage) and remembered on the session so the fast path can
	// short-circuit subsequent requests.
	hash := hashFieldset(visibleFields)
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

	// The client may have sent a hash that matches the freshly computed one
	// (e.g. its session cache was cold but its localStorage is current) — still 304.
	if clientHash != "" && clientHash == hash {
		w.WriteHeader(http.StatusNotModified)
		return
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
func hashFieldset(fields []field.Field) string {
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
func resolveFieldOptions(f field.Field) field.Field {
	opts := f.Options
	if opts == nil {
		return f
	}

	widget, _ := opts["widget"].(string)
	if f.Type != field.TYPE_SELECT && f.Type != field.TYPE_SELECT_ADDNEW && widget != "select" {
		return f
	}

	table, _ := opts["sourceTable"].(string)
	if table == "" {
		if ds, ok := opts["dataSource"].(string); ok && ds != "static" && ds != "database" && ds != "query" {
			table = ds
		}
	}
	if table == "" {
		return f // options already provided statically, or nothing to resolve
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
		return f
	}

	options := selectOptionsFromTable(table, valueField, displayField)
	if options == nil {
		return f
	}

	// Copy the options map so we never mutate the shared module definition.
	newOpts := make(map[string]interface{}, len(opts)+1)
	for k, val := range opts {
		newOpts[k] = val
	}
	newOpts["options"] = options
	f.Options = newOpts
	return f
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
