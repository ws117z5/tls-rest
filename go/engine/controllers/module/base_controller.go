package module

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"tls-rest/go/engine/controllers/db/cache"
	. "tls-rest/go/engine/controllers/field"
	"tls-rest/go/engine/controllers/httpx"
	"tls-rest/go/engine/controllers/log"

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
	v := viewerForModule(r, bc.Module.ID)
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
			// Resolved so list cells can render a select value's label (e.g. the
			// user's group name instead of its id).
			"options": resolveFieldOptions(field).Options,
		}
	}

	return fieldset
}

// createFiltersMap describes the module's declared list filters so the frontend
// can render filter controls. Filters the viewer may not see (per field access)
// are omitted, mirroring the fieldset visibility rules.
func (bc *BaseController) createFiltersMap(r *http.Request) []map[string]interface{} {
	filters := []map[string]interface{}{}
	if bc.Module == nil || bc.Module.Filters == nil {
		return filters
	}

	v := viewerForModule(r, bc.Module.ID)
	for _, field := range bc.Module.Filters.Fields {
		if !v.fieldVisibleInSchema(field) {
			continue
		}
		filters = append(filters, map[string]interface{}{
			"name":    field.Name,
			"type":    field.Type,
			"label":   field.Label,
			"match":   field.FilterMatch(),
			"options": field.Options,
		})
	}

	return filters
}

// respondError writes a uniform JSON error body:
//
//	{ "status": <code>, "message": <text>, "log_id": <id> }
//
// Server-side (5xx) errors are logged to the event logger and the returned
// log_id lets a user find the full detail in the logs module. Client errors
// (4xx) are returned as-is with an empty log_id (nothing to look up).
func (bc *BaseController) respondError(w http.ResponseWriter, status int, message string, err error) {
	logID := ""
	if status >= 500 {
		errStr := ""
		if err != nil {
			errStr = err.Error()
		}
		modID := ""
		if bc.Module != nil {
			modID = bc.Module.ID
		}
		logID = log.LogErrorWithID(message, modID, errStr)
		ModuleLog.Errorf("%s [log_id=%s]: %v", message, logID, err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  status,
		"message": message,
		"log_id":  logID,
	})
}

// List handles GET requests for listing records with pagination and filtering
func (bc *BaseController) List(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers for API calls
	httpx.AllowOrigin(w, r) // reflect only allowlisted origins (credentialed CORS)
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
		bc.respondError(w, http.StatusInternalServerError, "Failed to fetch records", err)
		return
	}

	// Create compatibility response format for frontend. Fieldset is intentionally
	// NOT included here — the client fetches it once from the dedicated
	// /api/modules/{id}/fieldset endpoint (hash-cached) rather than on every data
	// reload. Filters keep returning their values for now.
	compatResponse := map[string]interface{}{
		"Data":       result.Data,
		"Filters":    bc.createFiltersMap(r),
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
	httpx.AllowOrigin(w, r) // reflect only allowlisted origins (credentialed CORS)
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
		bc.respondError(w, http.StatusInternalServerError, "Failed to fetch record", err)
		return
	}

	// Return single record with compatibility format
	if resultData, ok := result.Data.([]map[string]interface{}); ok && len(resultData) > 0 {
		compatResponse := map[string]interface{}{
			"Data": resultData[0], // Single record for view
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

	// BeforeFieldset hook: transform the raw body before any field processing.
	if bc.Module != nil && bc.Module.BeforeFieldset != nil {
		updated, herr := bc.Module.BeforeFieldset(r, data)
		if herr != nil {
			bc.respondError(w, http.StatusBadRequest, herr.Error(), nil)
			return
		}
		data = updated
	}

	// Validate required fields
	if err := bc.validateRequiredFields(data, MODE_SUBMIT); err != nil {
		bc.respondError(w, http.StatusBadRequest, err.Error(), nil)
		return
	}

	// Filter data to only include valid fields
	filteredData := bc.filterValidFields(r, data, MODE_SUBMIT)

	// Apply field defaults for anything the client didn't submit, so a field left
	// at its default value is still persisted (create only).
	for _, field := range bc.Module.Fields {
		if field.Virtual || field.ReadOnly || field.DefaultValue == nil {
			continue
		}
		if _, ok := filteredData[field.Name]; !ok {
			filteredData[field.Name] = field.DefaultValue
		}
	}

	// Generate the uuid system value on create so the insert always satisfies the
	// uuid NOT NULL column (the engine doesn't rely on a DB default for it).
	if bc.Engine != nil && bc.Engine.hasField("uuid") {
		if _, ok := filteredData["uuid"]; !ok {
			filteredData["uuid"] = uuid.NewString()
		}
	}

	// Stamp the creating user. created_by is a system field the client never
	// submits; fill it from the session when the table has the column.
	if s := cache.SessionFromContext(r.Context()); s != nil && s.UserID > 0 {
		if bc.Engine != nil && bc.Engine.hasField("created_by") {
			filteredData["created_by"] = s.UserID
		}
	}

	// AfterFieldset hook: last chance to transform data before the DB write.
	if bc.Module != nil && bc.Module.AfterFieldset != nil {
		updated, herr := bc.Module.AfterFieldset(r, filteredData)
		if herr != nil {
			bc.respondError(w, http.StatusBadRequest, herr.Error(), nil)
			return
		}
		filteredData = updated
	}

	// Insert record
	id, err := bc.insertRecord(filteredData)
	if err != nil {
		bc.respondError(w, http.StatusInternalServerError, "Failed to create record", err)
		return
	}
	bc.notifyRightsChange()

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

	// BeforeFieldset hook: transform the raw body before any field processing.
	if bc.Module != nil && bc.Module.BeforeFieldset != nil {
		updated, herr := bc.Module.BeforeFieldset(r, data)
		if herr != nil {
			bc.respondError(w, http.StatusBadRequest, herr.Error(), nil)
			return
		}
		data = updated
	}

	// Filter data to only include valid fields (excluding read-only fields)
	filteredData := bc.filterValidFields(r, data, MODE_SUBMIT)

	// AfterFieldset hook: last chance to transform data before the DB write.
	if bc.Module != nil && bc.Module.AfterFieldset != nil {
		updated, herr := bc.Module.AfterFieldset(r, filteredData)
		if herr != nil {
			bc.respondError(w, http.StatusBadRequest, herr.Error(), nil)
			return
		}
		filteredData = updated
	}

	// Update record
	err := bc.updateRecord(id, filteredData)
	if err != nil {
		bc.respondError(w, http.StatusInternalServerError, "Failed to update record", err)
		return
	}
	bc.notifyRightsChange()

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
		bc.respondError(w, http.StatusInternalServerError, "Failed to delete record", err)
		return
	}
	bc.notifyRightsChange()

	response := map[string]interface{}{
		"id":      id,
		"message": "Record deleted successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// notifyRightsChange fires the OnRightsChange callback for RightsAffecting
// modules, so cached session rights are invalidated after users/groups/rights
// are created, edited or deleted. It also fires OnConfigChange for
// ConfigAffecting modules.
func (bc *BaseController) notifyRightsChange() {
	if bc.Module == nil {
		return
	}
	if bc.Module.RightsAffecting && OnRightsChange != nil {
		OnRightsChange()
	}
	if bc.Module.ConfigAffecting && OnConfigChange != nil {
		OnConfigChange()
	}
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
	v := viewerForModule(r, bc.Module.ID)

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
			// TYPE_TABLE with a submit hook: fold the submitted rows into the
			// stored value (e.g. checkbox rows -> {"field":["view","edit"]}).
			if field.TableSubmitFunc != nil {
				filtered[field.Name] = field.TableSubmitFunc(tableRows(value))
			} else {
				filtered[field.Name] = value
			}
		}
	}

	return filtered
}

// tableRows normalizes a submitted TYPE_TABLE value (a JSON string, []byte, or
// already-decoded slice) into a slice of row maps for TableSubmitFunc.
func tableRows(value interface{}) []map[string]interface{} {
	var raw []interface{}
	switch t := value.(type) {
	case string:
		if strings.TrimSpace(t) == "" {
			return nil
		}
		_ = json.Unmarshal([]byte(t), &raw)
	case []byte:
		_ = json.Unmarshal(t, &raw)
	case []interface{}:
		raw = t
	case []map[string]interface{}:
		return t
	}
	rows := make([]map[string]interface{}, 0, len(raw))
	for _, r := range raw {
		if m, ok := r.(map[string]interface{}); ok {
			rows = append(rows, m)
		}
	}
	return rows
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