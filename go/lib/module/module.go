package module

import (
	"errors"
	"fmt"
	stdlog "log"
	"net/http"
	"strings"
	"time"

	"tls-rest/go/lib/db/pgdb"

	"tls-rest/go/lib/log"

	"github.com/gorilla/mux"
)

const (
	// Legacy rights constants (for backward compatibility)
	RIGHT_NONE   = 0
	RIGHT_VIEW   = 1 << 0 // 1 - View rights
	RIGHT_CREATE = 1 << 1 // 2 - Create rights
	RIGHT_EDIT   = 1 << 2 // 4 - Edit rights
	RIGHT_DELETE = 1 << 3 // 8 - Delete rights
	RIGHT_ADMIN  = 1 << 4 // 16 - Admin rights
	RIGHT_ALL    = RIGHT_VIEW | RIGHT_CREATE | RIGHT_EDIT | RIGHT_DELETE | RIGHT_ADMIN

	// New permission system constants (matching auth/rights.go)
	PERMISSION_INHERIT = -1 // Inherit permission from parent (group or default)
	PERMISSION_DENY    = 0  // No access to module
	PERMISSION_READ    = 1  // Read-only access
	PERMISSION_WRITE   = 2  // Full read/write access
)

// ModuleHandler defines the interface for modules that want to override default behavior
type ModuleHandler interface {
	List(w http.ResponseWriter, r *http.Request)
	View(w http.ResponseWriter, r *http.Request)
	Create(w http.ResponseWriter, r *http.Request)
	Edit(w http.ResponseWriter, r *http.Request)
	Delete(w http.ResponseWriter, r *http.Request)
}

// Module represents a business module in your system.
type ModuleAbstract[T any] struct {
	ID                string
	Name              string
	Fields            []Field
	Rights            map[int]int // Legacy rights system (deprecated)
	Data              []T
	DefaultPermission int // Default permission level for new rights system

	// Optional custom handlers - if nil, will use default behavior
	CustomHandler ModuleHandler
	Controller    *BaseController // Exported for access by custom handlers
}

func (m *ModuleAbstract[T]) GetID() string {
	return m.ID
}

func (m *ModuleAbstract[T]) GetName() string {
	return m.Name
}

func (m *ModuleAbstract[T]) GetFields() []Field {
	return m.Fields
}

func (m *ModuleAbstract[T]) AddField(field Field) {
	m.Fields = append(m.Fields, field)
}

// ModuleInterface allows us to work with modules regardless of their generic type
type ModuleInterface interface {
	GetID() string
	GetName() string
	GetFields() []Field
	List(w http.ResponseWriter, r *http.Request)
	View(w http.ResponseWriter, r *http.Request)
	Create(w http.ResponseWriter, r *http.Request)
	Edit(w http.ResponseWriter, r *http.Request)
	Delete(w http.ResponseWriter, r *http.Request)
}

// Global registry for modules
var RegisteredModules = make(map[string]ModuleInterface)

// Global router for automatic route registration
var GlobalRouter *mux.Router

// Flag to track if auto-registration is enabled
var AutoRegisterRoutes = true

// Registry for module default permissions (matches auth package)
var ModuleDefaultPermissions = make(map[string]int)

// Event logging structure
type ModuleEvent struct {
	Timestamp  time.Time `json:"timestamp"`
	ModuleID   string    `json:"module_id"`
	Action     string    `json:"action"`
	UserID     string    `json:"user_id,omitempty"`
	RecordID   string    `json:"record_id,omitempty"`
	Details    string    `json:"details,omitempty"`
	Success    bool      `json:"success"`
	Error      string    `json:"error,omitempty"`
	Duration   int64     `json:"duration_ms,omitempty"`
	RemoteAddr string    `json:"remote_addr,omitempty"`
	UserAgent  string    `json:"user_agent,omitempty"`
}

// Module event logger
var ModuleLogger = stdlog.New(stdlog.Writer(), "[MODULE] ", stdlog.LstdFlags|stdlog.Lshortfile)

// RegisterModuleDefaultPermission sets the default permission for a module
func RegisterModuleDefaultPermission(module string, defaultPermission int) {
	ModuleDefaultPermissions[module] = defaultPermission
	ModuleLogger.Printf("Default permission %d registered for module %s", defaultPermission, module)
}

// SetModuleDefaultPermission sets default permission for a module instance
func (m *ModuleAbstract[T]) SetDefaultPermission(permission int) {
	m.DefaultPermission = permission
	RegisterModuleDefaultPermission(m.ID, permission)
}

// Helper functions to extract user and session info from request
func getSessionIDFromRequest(r *http.Request) string {
	// Try to get session ID from cookie
	if cookie, err := r.Cookie("session_id"); err == nil {
		return cookie.Value
	}

	// Try to get from custom header
	if sessionID := r.Header.Get("X-Session-ID"); sessionID != "" {
		return sessionID
	}

	// Generate a basic session ID from IP and UserAgent
	return r.RemoteAddr + "_" + strings.ReplaceAll(r.UserAgent(), " ", "_")
}

func getUserIDFromRequest(r *http.Request) *int {
	// In a real implementation, this would extract from context or session
	// For now, return nil (anonymous user)
	return nil
}

// LogModuleEvent logs module events with structured data
func LogModuleEvent(event ModuleEvent) {
	status := "SUCCESS"
	if !event.Success {
		status = "FAILED"
	}

	logMsg := fmt.Sprintf("%s - Module: %s, Action: %s", status, event.ModuleID, event.Action)

	if event.RecordID != "" {
		logMsg += fmt.Sprintf(", RecordID: %s", event.RecordID)
	}

	if event.UserID != "" {
		logMsg += fmt.Sprintf(", UserID: %s", event.UserID)
	}

	if event.Duration > 0 {
		logMsg += fmt.Sprintf(", Duration: %dms", event.Duration)
	}

	if event.RemoteAddr != "" {
		logMsg += fmt.Sprintf(", IP: %s", event.RemoteAddr)
	}

	if event.Error != "" {
		logMsg += fmt.Sprintf(", Error: %s", event.Error)
	}

	if event.Details != "" {
		logMsg += fmt.Sprintf(", Details: %s", event.Details)
	}

	ModuleLogger.Println(logMsg)
}

// Helper function to create event from HTTP request
func NewModuleEventFromRequest(moduleID, action string, r *http.Request) ModuleEvent {
	event := ModuleEvent{
		Timestamp:  time.Now(),
		ModuleID:   moduleID,
		Action:     action,
		RemoteAddr: r.RemoteAddr,
		UserAgent:  r.UserAgent(),
		Success:    true, // Will be set to false if error occurs
	}

	// Try to extract user ID from various sources
	if userID := extractUserID(r); userID != "" {
		event.UserID = userID
	}

	return event
}

// extractUserID tries to extract user ID from request headers, context, or session
func extractUserID(r *http.Request) string {
	// Check Authorization header for JWT or similar
	if auth := r.Header.Get("Authorization"); auth != "" {
		// This is a placeholder - implement actual JWT/token parsing as needed
		if strings.HasPrefix(auth, "Bearer ") {
			return "user_from_token" // Replace with actual token parsing
		}
	}

	// Check for user ID in headers
	if userID := r.Header.Get("X-User-ID"); userID != "" {
		return userID
	}

	// Check context for user information (if middleware sets it)
	if ctx := r.Context(); ctx != nil {
		if userID := ctx.Value("userID"); userID != nil {
			if uid, ok := userID.(string); ok {
				return uid
			}
		}
	}

	return "" // No user ID found
}

// Initialize the module with a controller (should be called after module definition)
func (m *ModuleAbstract[T]) Initialize(tableName string) {
	startTime := time.Now()

	// Log module initialization start
	event := ModuleEvent{
		Timestamp: startTime,
		ModuleID:  m.ID,
		Action:    "INITIALIZE",
		Details:   fmt.Sprintf("TableName: %s, FieldCount: %d", tableName, len(m.Fields)),
		Success:   true,
	}

	defer func() {
		if r := recover(); r != nil {
			event.Success = false
			event.Error = fmt.Sprintf("PANIC during initialization: %v", r)
			event.Duration = time.Since(startTime).Milliseconds()
			LogModuleEvent(event)
			ModuleLogger.Printf("PANIC in module %s initialization: %v", m.ID, r)
			panic(r) // Re-panic to maintain original behavior
		} else {
			event.Duration = time.Since(startTime).Milliseconds()
			LogModuleEvent(event)
		}
	}()

	// Inject default fields if not present
	addDefaultFields := func(fields []Field) []Field {
		defaultFields := []Field{
			{
				Name:       "id",
				Type:       "int",
				Label:      "ID",
				Required:   true,
				Options:    map[string]interface{}{"primary": true, "auto": true},
				Validation: map[string]interface{}{"min": 1},
			},
			{
				Name:     "uuid",
				Type:     "string",
				Label:    "UUID",
				Required: true,
				Options:  map[string]interface{}{"unique": true, "auto": true},
			},
			{
				Name:     "created",
				Type:     "datetime",
				Label:    "Created",
				Required: true,
				ReadOnly: true,
				Options:  map[string]interface{}{"auto": true},
			},
			{
				Name:     "updated",
				Type:     "datetime",
				Label:    "Updated",
				Required: true,
				ReadOnly: true,
				Options:  map[string]interface{}{"auto": true},
			},
			{
				Name:     "created_by",
				Type:     "int",
				Label:    "Created By",
				Required: false,
				Options:  map[string]interface{}{"auto": true},
			},
		}
		fieldMap := map[string]bool{}
		for _, f := range fields {
			fieldMap[f.Name] = true
		}
		for i := len(defaultFields) - 1; i >= 0; i-- {
			df := defaultFields[i]
			if !fieldMap[df.Name] {
				fields = append([]Field{df}, fields...)
			}
		}
		return fields
	}
	m.Fields = addDefaultFields(m.Fields)

	// Validation checks
	if m == nil {
		panic("module is nil")
	}
	if m.ID == "" {
		panic("module ID is empty")
	}
	if tableName == "" {
		panic("table name is empty")
	}

	// Create a non-generic wrapper for the controller
	ModuleLogger.Printf("Creating controller wrapper for module: %s", m.ID)

	moduleWrapper := &ModuleAbstract[interface{}]{
		ID:     m.ID,
		Name:   m.Name,
		Fields: m.Fields,
		Rights: m.Rights,
	}

	ModuleLogger.Printf("Creating base controller for module: %s", m.ID)
	m.Controller = NewBaseController(moduleWrapper, tableName)

	if m.Controller == nil {
		panic(fmt.Sprintf("failed to create controller for module %s", m.ID))
	}

	// Register globally for automatic routing
	ModuleLogger.Printf("Registering module globally: %s", m.ID)
	RegisteredModules[m.ID] = m

	// Register default permission for rights system
	if m.DefaultPermission == 0 {
		m.DefaultPermission = PERMISSION_READ // Default to read access if not set
	}
	RegisterModuleDefaultPermission(m.ID, m.DefaultPermission)

	// Automatically register routes if GlobalRouter is available and auto-registration is enabled
	if GlobalRouter != nil && AutoRegisterRoutes {
		ModuleLogger.Printf("Auto-registering routes for module: %s", m.ID)
		registerSingleModuleRoutes(GlobalRouter, m)
	}

	if GlobalFieldsetHandler != nil {
		ModuleLogger.Printf("Registering module with fieldset handler: %s", m.ID)
		GlobalFieldsetHandler.RegisterModule(moduleWrapper)
	} else {
		ModuleLogger.Printf("WARNING: GlobalFieldsetHandler is nil for module: %s", m.ID)
	}
}

// Default CRUD methods - can be overridden by setting CustomHandler
func (m *ModuleAbstract[T]) List(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	// Log module event using new logging system
	sessionID := getSessionIDFromRequest(r)
	userID := getUserIDFromRequest(r)

	log.LogModuleEvent(m.ID, "list", fmt.Sprintf("Module %s list operation started", m.ID), userID, sessionID, map[string]interface{}{
		"fields_count":       len(m.Fields),
		"has_custom_handler": m.CustomHandler != nil,
	})

	defer func() {
		duration := time.Since(startTime).Seconds() * 1000
		// Capture any panic/error
		if err := recover(); err != nil {
			log.LogError(
				fmt.Sprintf("Module %s list operation failed", m.ID),
				fmt.Sprintf("panic: %v", err),
				"", // stack trace would be added here in production
				map[string]interface{}{
					"module":      m.ID,
					"duration_ms": duration,
				},
			)
			panic(err) // Re-panic to maintain original behavior
		}

		log.LogModuleEvent(m.ID, "list", fmt.Sprintf("Module %s list operation completed", m.ID), userID, sessionID, map[string]interface{}{
			"duration_ms": duration,
			"success":     true,
		})
	}()

	if m.CustomHandler != nil {
		m.CustomHandler.List(w, r)
		return
	}
	if m.Controller != nil {
		m.Controller.List(w, r)
	}
}

func (m *ModuleAbstract[T]) View(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	event := NewModuleEventFromRequest(m.ID, "VIEW", r)

	// Extract record ID from URL path
	if vars := mux.Vars(r); vars != nil {
		if id, exists := vars["id"]; exists {
			event.RecordID = id
		}
	}

	defer func() {
		event.Duration = time.Since(startTime).Milliseconds()
		LogModuleEvent(event)
	}()

	if m.CustomHandler != nil {
		m.CustomHandler.View(w, r)
		return
	}
	if m.Controller != nil {
		m.Controller.View(w, r)
	}
}

func (m *ModuleAbstract[T]) Create(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	event := NewModuleEventFromRequest(m.ID, "CREATE", r)

	defer func() {
		event.Duration = time.Since(startTime).Milliseconds()
		LogModuleEvent(event)
	}()

	if m.CustomHandler != nil {
		m.CustomHandler.Create(w, r)
		return
	}
	if m.Controller != nil {
		m.Controller.Create(w, r)
	}
}

func (m *ModuleAbstract[T]) Edit(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	event := NewModuleEventFromRequest(m.ID, "EDIT", r)

	// Extract record ID from URL path
	if vars := mux.Vars(r); vars != nil {
		if id, exists := vars["id"]; exists {
			event.RecordID = id
		}
	}

	defer func() {
		event.Duration = time.Since(startTime).Milliseconds()
		LogModuleEvent(event)
	}()

	if m.CustomHandler != nil {
		m.CustomHandler.Edit(w, r)
		return
	}
	if m.Controller != nil {
		m.Controller.Edit(w, r)
	}
}

func (m *ModuleAbstract[T]) Delete(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	event := NewModuleEventFromRequest(m.ID, "DELETE", r)

	// Extract record ID from URL path
	if vars := mux.Vars(r); vars != nil {
		if id, exists := vars["id"]; exists {
			event.RecordID = id
		}
	}

	defer func() {
		event.Duration = time.Since(startTime).Milliseconds()
		LogModuleEvent(event)
	}()

	if m.CustomHandler != nil {
		m.CustomHandler.Delete(w, r)
		return
	}
	if m.Controller != nil {
		m.Controller.Delete(w, r)
	}
}

func (m *ModuleAbstract[T]) getQuery() string {
	query := "SELECT "
	for i, field := range m.Fields {
		if field.SQL != "" {
			query += field.getSQL()
		} else {
			query += field.Name
		}
		if i < len(m.Fields)-1 {
			query += ", "
		}
	}
	query += " FROM " + m.ID

	return query
}

func (m *ModuleAbstract[T]) HasRight(userGroupID int, right int) bool {
	if r, ok := m.Rights[userGroupID]; ok {
		return r&right != 0
	}
	return false
}

func (m *ModuleAbstract[T]) SetRight(userGroupID, moduleID, right int) {
	if r, ok := m.Rights[userGroupID]; ok {
		m.Rights[userGroupID] = r | right
	} else {
		m.Rights[userGroupID] = right
	}
}

// getDB returns a database instance
func (m *ModuleAbstract[T]) getDB() (*pgdb.Db, error) {
	return pgdb.GetInstance()
}

// In-memory store (replace with DB in production)
var (
	modules = make(map[string]*ModuleAbstract[any]) // map of module ID to Module
	//modulesMu sync.RWMutex
)

// CreateModule creates a new module.
func CreateModule[T any](id, name string) (*ModuleAbstract[T], error) {
	if _, exists := modules[id]; exists {
		return nil, errors.New("module already exists")
	}
	m := &ModuleAbstract[T]{
		ID:     id,
		Name:   name,
		Fields: []Field{},
		//Filterable: []string{},
		Rights: make(map[int]int),
	}
	modules[id] = any(m).(*ModuleAbstract[any])
	return m, nil
}

// AddField adds a field to a module.
func AddField(moduleID string, field Field) error {
	m, ok := modules[moduleID]
	if !ok {
		return errors.New("module not found")
	}
	m.Fields = append(m.Fields, field)
	return nil
}

// EditField edits a field in a module.
func EditField(moduleID, fieldName string, newField Field) error {
	m, ok := modules[moduleID]
	if !ok {
		return errors.New("module not found")
	}
	for i, f := range m.Fields {
		if f.Name == fieldName {
			m.Fields[i] = newField
			return nil
		}
	}
	return errors.New("field not found")
}

// DeleteField removes a field from a module.
func DeleteField(moduleID, fieldName string) error {
	m, ok := modules[moduleID]
	if !ok {
		return errors.New("module not found")
	}
	for i, f := range m.Fields {
		if f.Name == fieldName {
			m.Fields = append(m.Fields[:i], m.Fields[i+1:]...)
			return nil
		}
	}
	return errors.New("field not found")
}

// SetFilterableFields sets which fields are filterable in list view.
// func SetFilterableFields(moduleID string, fields []string) error {
// 	modulesMu.Lock()
// 	defer modulesMu.Unlock()
// 	m, ok := modules[moduleID]
// 	if !ok {
// 		return errors.New("module not found")
// 	}
// 	m.Filterable = fields
// 	return nil
// }

// GetModule fetches a module by ID from the database.
// func GetModules(id string, db *pgdb.Db) (*ModuleAbstract[any], error) {

// 	data, err := db.GetAll("modules", []string{"id", "name"}, map[string]string{"id": id})

// 	if err != nil {
// 		return nil, err
// 	}

// 	if len(data) == 0 {
// 		return nil, errors.New("module not found")
// 	}

// 	var m ModuleAbstract[any]
// 	if rows.Next() {
// 		if err := rows.Scan(&m.ID, &m.Name); err != nil {
// 			return nil, err
// 		}
// 	} else {
// 		return nil, errors.New("module not found")
// 	}

// 	// Fetch fields for the module
// 	fieldsQuery := "SELECT name, type, required FROM module_fields WHERE module_id = ?"
// 	fieldRows, err := db.Query(fieldsQuery, id)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer fieldRows.Close()

// 	for fieldRows.Next() {
// 		var f Field
// 		if err := fieldRows.Scan(&f.Name, &f.Type, &f.Required); err != nil {
// 			return nil, err
// 		}
// 		m.Fields = append(m.Fields, f)
// 	}

// 	// Fetch filterable fields
// 	filterQuery := "SELECT field_name FROM module_filterable WHERE module_id = ?"
// 	filterRows, err := db.Query(filterQuery, id)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer filterRows.Close()

// 	for filterRows.Next() {
// 		var fname string
// 		if err := filterRows.Scan(&fname); err != nil {
// 			return nil, err
// 		}
// 		m.Filterable = append(m.Filterable, fname)
// 	}

// 	// Fetch rights for users
// 	m.Rights = make(map[string]int)
// 	rightsQuery := "SELECT user_id, rights FROM user_rights WHERE module_id = ?"
// 	rightsRows, err := db.Query(rightsQuery, id)
// 	if err == nil {
// 		defer rightsRows.Close()
// 		for rightsRows.Next() {
// 			var userID string
// 			var rights int
// 			if err := rightsRows.Scan(&userID, &rights); err == nil {
// 				m.Rights[userID] = rights
// 			}
// 		}
// 	}

// 	// Fetch rights for groups
// 	groupRightsQuery := "SELECT group_id, rights FROM user_group_rights WHERE module_id = ?"
// 	groupRightsRows, err := db.Query(groupRightsQuery, id)
// 	if err == nil {
// 		defer groupRightsRows.Close()
// 		for groupRightsRows.Next() {
// 			var groupID string
// 			var rights int
// 			if err := groupRightsRows.Scan(&groupID, &rights); err == nil {
// 				m.Rights[groupID] = rights
// 			}
// 		}
// 	}

// 	return &m, nil
// }

// SetModuleRights sets rights for a user or group on a module in the database.
func SetModuleRights(moduleID, userOrGroupID string, right int, isGroup bool, db *pgdb.Db) error {
	var query string
	if isGroup {
		query = "INSERT INTO user_group_rights (module_id, group_id, rights) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE rights = ?"
		_, err := db.Exec(query, moduleID, userOrGroupID, right, right)
		return err
	} else {
		query = "INSERT INTO user_rights (module_id, user_id, rights) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE rights = ?"
		_, err := db.Exec(query, moduleID, userOrGroupID, right, right)
		return err
	}
}

// fieldTypeToSQL converts a field type to SQL column definition
func (m *ModuleAbstract[T]) fieldTypeToSQL(field Field) string {
	var sqlType string

	switch field.Type {
	case TYPE_INT:
		sqlType = "INTEGER"
	case TYPE_FLOAT:
		sqlType = "NUMERIC"
	case TYPE_STRING:
		sqlType = "VARCHAR(255)"
	case TYPE_TEXT, TYPE_HTML:
		sqlType = "TEXT"
	case TYPE_DATE:
		sqlType = "DATE"
	case TYPE_DATE_TIME:
		sqlType = "TIMESTAMP WITH TIME ZONE"
	case TYPE_CHECKBOX, TYPE_YES_NO:
		sqlType = "BOOLEAN"
	case TYPE_JSON:
		sqlType = "JSONB"
	case TYPE_TABLE:
		// For table fields, check the data source type
		if field.Options != nil {
			if dataSource, ok := field.Options["dataSource"].(string); ok {
				switch dataSource {
				case "database", "query":
					// For database tables, this field is virtual - no storage needed
					return "" // Empty means this field won't be created in the table
				case "static":
					sqlType = "JSONB" // Store static data as JSON
				default:
					sqlType = "JSONB" // Default fallback
				}
			} else {
				sqlType = "JSONB" // Default fallback
			}
		} else {
			sqlType = "JSONB" // Default fallback
		}
	case TYPE_MONEY:
		sqlType = "NUMERIC(15,2)"
	default:
		sqlType = "TEXT" // Default fallback
	}

	// Add NOT NULL constraint if required
	if field.Required {
		sqlType += " NOT NULL"
	}

	// Add default value if specified
	if field.SQL != "" {
		// If field has custom SQL (like uuid_generate_v4()), use it as default
		sqlType += fmt.Sprintf(" DEFAULT %s", field.SQL)
	} else if field.DefaultValue != nil {
		// Use the default value specified
		switch v := field.DefaultValue.(type) {
		case string:
			sqlType += fmt.Sprintf(" DEFAULT '%s'", v)
		case int, int64, float64:
			sqlType += fmt.Sprintf(" DEFAULT %v", v)
		case bool:
			sqlType += fmt.Sprintf(" DEFAULT %t", v)
		}
	}

	return sqlType
}

// EnsureTableExists creates the table if it doesn't exist based on module fieldset
func (m *ModuleAbstract[T]) EnsureTableExists() error {
	startTime := time.Now()

	event := ModuleEvent{
		Timestamp: startTime,
		ModuleID:  m.ID,
		Action:    "TABLE_CHECK",
		Success:   true,
	}

	defer func() {
		event.Duration = time.Since(startTime).Milliseconds()
		LogModuleEvent(event)
	}()

	db, err := m.getDB()
	if err != nil {
		event.Success = false
		event.Error = fmt.Sprintf("failed to get database connection: %v", err)
		return fmt.Errorf("failed to get database connection: %w", err)
	}

	// Check if table exists
	exists, err := db.TableExists(m.ID)
	if err != nil {
		event.Success = false
		event.Error = fmt.Sprintf("failed to check if table exists: %v", err)
		return fmt.Errorf("failed to check if table exists: %w", err)
	}

	if exists {
		event.Details = "Table already exists"
		return nil // Table already exists
	}

	// Table doesn't exist, need to create it
	event.Action = "TABLE_CREATE"
	event.Details = fmt.Sprintf("Creating table with %d fields", len(m.Fields))

	// Create sequence for id field FIRST if it exists and is integer type
	for _, field := range m.Fields {
		if field.Name == "id" && field.Type == TYPE_INT {
			sequenceSQL := fmt.Sprintf(`CREATE SEQUENCE IF NOT EXISTS %s_id_seq`, m.ID)
			_, err = db.Query(sequenceSQL)
			if err != nil {
				return fmt.Errorf("failed to create sequence for table %s: %w", m.ID, err)
			}
			break
		}
	}

	// Generate CREATE TABLE statement
	createSQL := m.generateCreateTableSQL()

	// Execute the CREATE TABLE statement
	_, err = db.Query(createSQL)
	if err != nil {
		event.Success = false
		event.Error = fmt.Sprintf("failed to create table %s: %v", m.ID, err)
		return fmt.Errorf("failed to create table %s: %w", m.ID, err)
	}

	event.Details += " - SUCCESS"
	return nil
}

// generateCreateTableSQL generates the CREATE TABLE SQL statement
func (m *ModuleAbstract[T]) generateCreateTableSQL() string {
	var columns []string
	var primaryKeys []string

	for _, field := range m.Fields {
		if field.Virtual {
			continue // Skip virtual fields
		}

		sqlType := m.fieldTypeToSQL(field)
		if sqlType == "" {
			continue // Skip fields that don't need database storage (like database table references)
		}

		columnDef := fmt.Sprintf("%s %s", field.Name, sqlType)

		// Handle special cases
		switch field.Name {
		case "id":
			if field.Type == TYPE_INT {
				columnDef = fmt.Sprintf("id INTEGER NOT NULL DEFAULT nextval('%s_id_seq'::regclass)", m.ID)
			}
			primaryKeys = append(primaryKeys, field.Name)
		case "uuid":
			if field.Type == TYPE_STRING && field.SQL != "" {
				columnDef = "uuid UUID NOT NULL DEFAULT uuid_generate_v4()"
				// If no id field, use uuid as primary key
			}
			// Check if id field exists, if not use uuid as primary key
			hasIdField := false
			for _, f := range m.Fields {
				if f.Name == "id" {
					hasIdField = true
					break
				}
			}
			if !hasIdField {
				primaryKeys = append(primaryKeys, field.Name)
			}
		case "created", "created_at":
			if field.Type == TYPE_DATE_TIME {
				columnDef = fmt.Sprintf("%s TIMESTAMP WITH TIME ZONE DEFAULT now()", field.Name)
			}
		case "updated", "updated_at":
			if field.Type == TYPE_DATE_TIME {
				columnDef = fmt.Sprintf("%s TIMESTAMP WITH TIME ZONE DEFAULT now()", field.Name)
			}
		}

		columns = append(columns, columnDef)
	}

	// Build the CREATE TABLE statement
	sql := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n    %s", m.ID, strings.Join(columns, ",\n    "))

	// Add primary key constraint
	if len(primaryKeys) > 0 {
		sql += fmt.Sprintf(",\n    CONSTRAINT %s_pkey PRIMARY KEY (%s)", m.ID, strings.Join(primaryKeys, ", "))
	}

	sql += "\n)"

	return sql
}

// Public methods for testing
func (m *ModuleAbstract[T]) GenerateCreateTableSQL() string {
	return m.generateCreateTableSQL()
}

func (m *ModuleAbstract[T]) FieldTypeToSQL(field Field) string {
	return m.fieldTypeToSQL(field)
}

// Rights management methods
func (m *ModuleAbstract[T]) RevokeRight(userOrGroupID, right int) {
	if r, ok := m.Rights[userOrGroupID]; ok {
		m.Rights[userOrGroupID] = r & ^right
	}
}

func (m *ModuleAbstract[T]) GetRights(userOrGroupID int) int {
	if r, ok := m.Rights[userOrGroupID]; ok {
		return r
	}
	return RIGHT_NONE
}

func HasRight(rights int, right int) bool {
	return (rights & right) != 0
}

func AddRight(rights int, right int) int {
	return rights | right
}

func RemoveRight(rights int, right int) int {
	return rights & ^right
}

func GetRightName(right int) string {
	switch right {
	case RIGHT_VIEW:
		return "View"
	case RIGHT_CREATE:
		return "Create"
	case RIGHT_EDIT:
		return "Edit"
	case RIGHT_DELETE:
		return "Delete"
	case RIGHT_ADMIN:
		return "Admin"
	default:
		return "Unknown"
	}
}

func GetRightNames(rights int) []string {
	var names []string
	if HasRight(rights, RIGHT_VIEW) {
		names = append(names, "View")
	}
	if HasRight(rights, RIGHT_CREATE) {
		names = append(names, "Create")
	}
	if HasRight(rights, RIGHT_EDIT) {
		names = append(names, "Edit")
	}
	if HasRight(rights, RIGHT_DELETE) {
		names = append(names, "Delete")
	}
	if HasRight(rights, RIGHT_ADMIN) {
		names = append(names, "Admin")
	}
	return names
}

// registerSingleModuleRoutes registers routes for a single module
func registerSingleModuleRoutes(router *mux.Router, module ModuleInterface) {
	moduleID := module.GetID()
	ModuleLogger.Printf("Registering routes for module: %s", moduleID)

	// Create subrouter for this module
	subrouter := router.PathPrefix("/" + moduleID).Subrouter()

	// Register CRUD routes
	subrouter.HandleFunc("", module.List).Methods("GET")
	subrouter.HandleFunc("", module.Create).Methods("POST")
	subrouter.HandleFunc("/{id}", module.View).Methods("GET")
	subrouter.HandleFunc("/{id}", module.Edit).Methods("PUT", "PATCH")
	subrouter.HandleFunc("/{id}", module.Delete).Methods("DELETE")

	ModuleLogger.Printf("Routes registered for module %s: GET,POST /%s, GET,PUT,PATCH,DELETE /%s/{id}",
		moduleID, moduleID, moduleID)
}

// RegisterModuleRoutes automatically registers CRUD routes for all registered modules
func RegisterModuleRoutes(router *mux.Router) {
	startTime := time.Now()
	moduleCount := len(RegisteredModules)

	ModuleLogger.Printf("Starting automatic route registration for %d modules", moduleCount)

	for _, module := range RegisteredModules {
		registerSingleModuleRoutes(router, module)
	}

	duration := time.Since(startTime).Milliseconds()
	ModuleLogger.Printf("Route registration completed in %dms. %d modules registered.", duration, moduleCount)
}

// SetGlobalRouter sets the global router for automatic route registration
func SetGlobalRouter(router *mux.Router) {
	GlobalRouter = router
	ModuleLogger.Printf("Global router set for automatic module route registration")

	// Register existing modules if any
	if len(RegisteredModules) > 0 {
		ModuleLogger.Printf("Registering %d existing modules with new global router", len(RegisteredModules))
		for _, module := range RegisteredModules {
			registerSingleModuleRoutes(router, module)
		}
	}
}

// EnableAutoRegistration enables or disables automatic route registration
func EnableAutoRegistration(enabled bool) {
	AutoRegisterRoutes = enabled
	if enabled {
		ModuleLogger.Printf("Automatic route registration ENABLED")
	} else {
		ModuleLogger.Printf("Automatic route registration DISABLED")
	}
}

// RegisterModuleByID manually registers routes for a specific module by ID
func RegisterModuleByID(router *mux.Router, moduleID string) error {
	module, exists := RegisteredModules[moduleID]
	if !exists {
		return fmt.Errorf("module '%s' not found in registry", moduleID)
	}

	registerSingleModuleRoutes(router, module)
	ModuleLogger.Printf("Manually registered routes for module: %s", moduleID)
	return nil
}

// GetRegisteredModuleIDs returns a list of all registered module IDs
func GetRegisteredModuleIDs() []string {
	ids := make([]string, 0, len(RegisteredModules))
	for id := range RegisteredModules {
		ids = append(ids, id)
	}
	return ids
}
