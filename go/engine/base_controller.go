package module

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

// BaseController provides common CRUD operations using fieldsets
type BaseController struct {
	Module    *ModuleAbstract[interface{}]
	TableName string
	Engine    *FieldsetEngine
}

// NewBaseController creates a new base controller instance
func NewBaseController(module *ModuleAbstract[interface{}], tableName string) *BaseController {
	engine := NewFieldsetEngine(module, tableName)
	return &BaseController{
		Module:    module,
		TableName: tableName,
		Engine:    engine,
	}
}

// createFieldsetMap creates a fieldset map for frontend compatibility, filtered
// to the fields the requesting user may see (system fields are admin-only;
// access-gated fields are hidden from lower levels).
func (bc *BaseController) createFieldsetMap(r *http.Request) map[string]interface{} {
	v := viewerFromRequest(r)
	fieldset := make(map[string]interface{})

	for _, field := range bc.Module.Fields {
		if field.Type == TYPE_TABLE || !v.fieldVisibleInSchema(field) {
			continue
		}
		fieldset[field.Name] = map[string]interface{}{
			"name":       field.Name,
			"type":       field.Type,
			"label":      field.Label,
			"required":   field.Required,
			"readonly":   field.ReadOnly,
			"virtual":    field.Virtual,
			"filterable": field.Filterable,
			"sortable":   field.Sortable,
			"searchable": field.Searchable,
			"mode":       field.Mode,
			"access":     field.Access,
		}
	}

	return fieldset
}

// List handles GET requests for listing records with pagination and filtering
func (bc *BaseController) List(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers for API calls
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-Type, Accept")

	// Handle preflight OPTIONS request
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	bc.Engine.WithRequest(r)

	result, err := bc.Engine.ExecuteQuery(MODE_LIST)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch records: %v", err), http.StatusInternalServerError)
		return
	}

	// Create compatibility response format for frontend
	compatResponse := map[string]interface{}{
		"Data":       result.Data,
		"Fieldset":   bc.createFieldsetMap(r),
		"Total":      result.Total,
		"Page":       result.Page,
		"Limit":      result.Limit,
		"TotalPages": result.TotalPages,
	}

	// Log the response for debugging
	if data, ok := result.Data.([]map[string]interface{}); ok {
		fmt.Printf("API Response for %s: Data count: %d, Total: %d\n", bc.Module.ID, len(data), result.Total)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(compatResponse)
}

// View handles GET requests for viewing a single record by ID
func (bc *BaseController) View(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers for API calls
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-Type, Accept")

	// Handle preflight OPTIONS request
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	if id == "" {
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}

	// Create a temporary fieldset engine with ID filter
	engine := bc.Engine.WithRequest(r)

	// Manually add ID filter to the request URL query
	q := r.URL.Query()
	q.Set("filters[id]", id)
	r.URL.RawQuery = q.Encode()

	result, err := engine.ExecuteQuery(MODE_VIEW)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch record: %v", err), http.StatusInternalServerError)
		return
	}

	// Return single record with compatibility format
	if resultData, ok := result.Data.([]map[string]interface{}); ok && len(resultData) > 0 {
		compatResponse := map[string]interface{}{
			"Data":     resultData[0], // Single record for view
			"Fieldset": bc.createFieldsetMap(r),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(compatResponse)
	} else {
		http.Error(w, "Record not found", http.StatusNotFound)
	}
}

// Create handles POST requests for creating new records
func (bc *BaseController) Create(w http.ResponseWriter, r *http.Request) {
	// Parse request body
	var data map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if err := bc.validateRequiredFields(data, MODE_EDIT); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Filter data to only include valid fields
	filteredData := bc.filterValidFields(r, data, MODE_EDIT)

	// Insert record
	id, err := bc.insertRecord(filteredData)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create record: %v", err), http.StatusInternalServerError)
		return
	}

	// Return created record
	response := map[string]interface{}{
		"id":      id,
		"message": "Record created successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// Edit handles PUT requests for updating existing records
func (bc *BaseController) Edit(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	if id == "" {
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}

	// Parse request body
	var data map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	// Filter data to only include valid fields (excluding read-only fields)
	filteredData := bc.filterValidFields(r, data, MODE_EDIT)

	// Update record
	err := bc.updateRecord(id, filteredData)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to update record: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"id":      id,
		"message": "Record updated successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Delete handles DELETE requests for removing records
func (bc *BaseController) Delete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	if id == "" {
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}

	err := bc.deleteRecord(id)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete record: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"id":      id,
		"message": "Record deleted successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Helper methods

func (bc *BaseController) validateRequiredFields(data map[string]interface{}, mode int) error {
	for _, field := range bc.Module.Fields {
		// Skip validation for virtual fields and fields not in current mode
		if field.Virtual || (field.Mode&mode == 0) {
			continue
		}

		if field.Required {
			if value, exists := data[field.Name]; !exists || value == nil || value == "" {
				return fmt.Errorf("field '%s' is required", field.Name)
			}
		}
	}
	return nil
}

func (bc *BaseController) filterValidFields(r *http.Request, data map[string]interface{}, mode int) map[string]interface{} {
	filtered := make(map[string]interface{})
	v := viewerFromRequest(r)

	for _, field := range bc.Module.Fields {
		// Skip virtual fields, read-only fields, and fields not in current mode.
		if field.Virtual || field.ReadOnly || (field.Mode&mode == 0) {
			continue
		}
		// A user may only write fields they are allowed to see: this stops a
		// non-admin from setting the admin-only access level (privilege
		// escalation) or any access-gated field.
		if !v.fieldVisibleInSchema(field) {
			continue
		}

		if value, exists := data[field.Name]; exists {
			filtered[field.Name] = value
		}
	}

	return filtered
}

func (bc *BaseController) insertRecord(data map[string]interface{}) (int64, error) {
	db, err := bc.Engine.Module.getDB()
	if err != nil {
		return 0, err
	}

	return db.InsertRow(bc.TableName, data)
}

func (bc *BaseController) updateRecord(id string, data map[string]interface{}) error {
	db, err := bc.Engine.Module.getDB()
	if err != nil {
		return err
	}

	// Convert id to appropriate type
	var idValue interface{} = id
	if idInt, err := strconv.ParseInt(id, 10, 64); err == nil {
		idValue = idInt
	}

	_, err = db.UpdateRow(bc.TableName, data, "id", idValue)
	return err
}

func (bc *BaseController) deleteRecord(id string) error {
	db, err := bc.Engine.Module.getDB()
	if err != nil {
		return err
	}

	// Convert id to appropriate type
	var idValue interface{} = id
	if idInt, err := strconv.ParseInt(id, 10, 64); err == nil {
		idValue = idInt
	}

	_, err = db.DeleteRow(bc.TableName, "id", idValue)
	return err
}
