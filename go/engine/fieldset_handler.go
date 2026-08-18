package module

import (
	"encoding/json"
	"fmt"
	"net/http"

	"tls-rest/go/lib/db/pgdb"

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

// GetFieldset handles GET /api/modules/{moduleId}/fieldset
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

	// Parse mode parameter
	modeParam := r.URL.Query().Get("mode")
	mode := MODE_ALL // Default to all modes
	if modeParam != "" {
		// You could parse mode from string representation if needed
		// For now, just use the default
	}

	// Filter fields based on mode and the requesting user's access (system
	// fields are admin-only; access-gated fields are hidden from lower levels).
	v := viewerFromRequest(r)
	var visibleFields []Field
	for _, field := range module.Fields {
		if (field.Mode == 0 || (field.Mode&mode) != 0) && v.fieldVisibleInSchema(field) {
			visibleFields = append(visibleFields, resolveFieldOptions(field))
		}
	}

	// Prepare response
	response := map[string]interface{}{
		"id":     module.ID,
		"name":   module.Name,
		"fields": visibleFields,
		"mode":   mode,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
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
func resolveFieldOptions(field Field) Field {
	opts := field.Options
	if opts == nil {
		return field
	}

	widget, _ := opts["widget"].(string)
	if field.Type != TYPE_SELECT && field.Type != TYPE_SELECT_ADDNEW && widget != "select" {
		return field
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
	rows, err := db.RQuery(query)
	if err != nil {
		return nil
	}
	options := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		options = append(options, map[string]interface{}{
			"value": row["value"],
			"name":  pgdb.AsString(row["label"]),
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
