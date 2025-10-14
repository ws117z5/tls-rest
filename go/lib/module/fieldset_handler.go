package module

import (
	"encoding/json"
	"net/http"

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

// RegisterModule registers a module for fieldset API
func (fh *FieldsetHandler) RegisterModule(module *ModuleAbstract[interface{}]) {
	fh.modules[module.ID] = module
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

	// Filter fields based on mode
	var visibleFields []Field
	for _, field := range module.Fields {
		// Include field if mode matches or no mode restriction
		if field.Mode == 0 || (field.Mode&mode) != 0 {
			visibleFields = append(visibleFields, field)
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

// Global fieldset handler instance
var GlobalFieldsetHandler = NewFieldsetHandler()
