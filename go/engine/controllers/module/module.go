package module

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"tls-rest/go/engine/controllers/db/pgdb"
	. "tls-rest/go/engine/controllers/field"
	"tls-rest/go/engine/controllers/log"

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

// HandlerOverrides lets a module replace individual engine handlers without
// implementing all of them (unlike CustomHandler, which is all-or-nothing). A
// nil field falls through to CustomHandler and then the default controller, so a
// module can override just one action (e.g. List) and keep the rest generic.
type HandlerOverrides struct {
	List   http.HandlerFunc
	View   http.HandlerFunc
	Create http.HandlerFunc
	Edit   http.HandlerFunc
	Delete http.HandlerFunc
}

// CustomRoute is an extra route a module registers beyond the standard CRUD
// (e.g. a binary upload or serve endpoint). When Absolute is true the Path is
// registered on the root router as-is; otherwise it is relative to the module's
// /<id> subrouter. Methods defaults to GET when empty.
type CustomRoute struct {
	Path     string
	Methods  []string
	Handler  http.HandlerFunc
	Absolute bool
}

// CustomRouter is implemented by modules that register routes beyond CRUD.
// registerSingleModuleRoutes detects it, so a module only needs to populate its
// CustomRoutes field (ModuleAbstract satisfies this automatically).
type CustomRouter interface {
	GetCustomRoutes() []CustomRoute
}

// Module represents a business module in your system.
type ModuleAbstract[T any] struct {
	ID   string
	Name string
	// Description and Order drive the menu entry (returned by /api/modules).
	// Order sorts the menu (lower first); Description is the menu tooltip/subtitle.
	Description string
	Order       int
	// Submenu groups this module under a named submenu in the menu (e.g.
	// "engine"); empty places it at the top level.
	Submenu  string
	Icon     string
	ReadOnly bool
	Hidden   bool
	// HiddenModes removes specific modes (e.g. "edit") from the menu without
	// making the module fully read-only — the buttons disappear but other modes
	// (like create) remain. Enforcement of the mode itself is via Overrides.
	HiddenModes []string
	Fields      []Field
	// Filters declares the list-mode filters this module accepts, as an ordinary
	// fieldset (conventionally defined in <module>/filters.go). When set, the
	// fieldset engine turns matching query parameters into SQL WHERE clauses for
	// List. Nil means the module declares no filters.
	Filters           *Filedset
	Rights            map[int]int // Legacy rights system (deprecated)
	Data              []T
	DefaultPermission int // Default permission level for new rights system
	// DefaultPermissionSet distinguishes an explicit default of 0 (PERMISSION_DENY,
	// e.g. admin-only modules) from an unset default. Without it, Initialize would
	// silently promote every 0 to PERMISSION_READ, making restricted modules world-
	// readable. Modules that want DENY set DefaultPermission: 0 and this flag: true.
	DefaultPermissionSet bool

	// OmitSystemFields lists engine-managed default fields the module does NOT
	// want auto-injected: any of "uuid", "created", "updated", "created_by",
	// "access" (and "id", though dropping id breaks CRUD-by-id — avoid). Use it
	// for modules mapping onto a purpose-built table that lacks those columns, so
	// generated SELECT/INSERT never reference them. Fields the module declares
	// itself are unaffected; a missing "access" column is treated as access 0 by
	// the row-level filter, so module-level DefaultPermission is what gates such
	// modules.
	OmitSystemFields []string

	// Optional custom handlers - if nil, will use default behavior
	CustomHandler ModuleHandler
	Controller    *BaseController // Exported for access by custom handlers

	// Overrides replaces individual engine handlers (granular; a nil field uses
	// the default). CustomRoutes adds endpoints beyond CRUD (e.g. binary
	// upload/serve). Both are honoured automatically on route registration.
	Overrides    HandlerOverrides
	CustomRoutes []CustomRoute

	// OwnerScoped restricts list/count results to rows the current user created:
	// a non-admin only sees records whose created_by matches their user id.
	// Requires a created_by column (present unless dropped via OmitSystemFields);
	// admins are not scoped. Use for per-user data such as a personal word list.
	OwnerScoped bool

	// BeforeFieldset and AfterFieldset are optional data hooks around field
	// processing on create/update, letting a module change the data before it is
	// written (the module-level analogue of the images Preprocessor).
	//   BeforeFieldset — receives the RAW decoded request body, before the
	//                    fieldset filters/coerces/validates fields.
	//   AfterFieldset  — receives the FILTERED data, just before the DB write.
	// Each returns the (possibly modified) data, or an error to abort with 400.
	BeforeFieldset Preprocessor
	AfterFieldset  Preprocessor
}

// Preprocessor transforms a submitted data map during create/update. Use it to
// derive, normalize, inject, or strip fields before persistence. r is the request
// (read mux.Vars for the id on update, or the session); data is the working map.
type Preprocessor func(r *http.Request, data map[string]interface{}) (map[string]interface{}, error)

func (m *ModuleAbstract[T]) GetID() string {
	return m.ID
}

// IsHidden / IsReadOnly are read from the always-populated RegisteredModules
// registry (see ModulesAPI), so they work regardless of whether the menu writer
// is wired.
func (m *ModuleAbstract[T]) IsHidden() bool {
	return m.Hidden
}
func (m *ModuleAbstract[T]) IsReadOnly() bool {
	return m.ReadOnly
}
func (m *ModuleAbstract[T]) GetHiddenModes() []string {
	return m.HiddenModes
}

// GetCustomRoutes exposes the module's extra routes (satisfies CustomRouter).
func (m *ModuleAbstract[T]) GetCustomRoutes() []CustomRoute {
	return m.CustomRoutes
}

func (m *ModuleAbstract[T]) GetName() string {
	return m.Name
}

func (m *ModuleAbstract[T]) GetFields() []Field {
	return m.Fields
}

// GetFilters returns the module's declared list-mode filters (may be nil).
func (m *ModuleAbstract[T]) GetFilters() *Filedset {
	return m.Filters
}

func (m *ModuleAbstract[T]) AddField(field Field) {
	m.Fields = append(m.Fields, field)
}

// ModuleInterface allows us to work with modules regardless of their generic type
type ModuleInterface interface {
	GetID() string
	GetName() string
	GetFields() []Field
	IsHidden() bool
	IsReadOnly() bool
	GetHiddenModes() []string
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
// Module lifecycle diagnostics go to the console-only logger (never file/db).
var ModuleLog = log.Console.With("module")

// RegisterModuleDefaultPermission sets the default permission for a module
func RegisterModuleDefaultPermission(module string, defaultPermission int) {
	ModuleDefaultPermissions[module] = defaultPermission
	ModuleLog.Debugf("Default permission %d registered for module %s", defaultPermission, module)
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

	ModuleLog.Debug(logMsg)
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
			ModuleLog.Errorf("PANIC in module %s initialization: %v", m.ID, r)
			panic(r) // Re-panic to maintain original behavior
		} else {
			event.Duration = time.Since(startTime).Milliseconds()
			LogModuleEvent(event)
		}
	}()

	// Inject default fields if not present.
	//
	// System fields (id/uuid/created/updated/created_by) are read-only and only
	// meaningful to admins: shown in list/view and shown read-only in edit, but
	// never in create (nothing to show for a record that doesn't exist yet) and
	// never editable. Admin-only visibility is enforced at request time by the
	// engine; the modes here only control which forms they can appear in.
	//
	// The access field is the per-record access level (0 = everyone). Unlike the
	// other system fields it is editable by admins (it is the control), so it also
	// appears in create.
	addDefaultFields := func(fields []Field) []Field {
		const sysMode = MODE_LIST | MODE_VIEW | MODE_EDIT
		defaultFields := []Field{
			{
				Name:       "id",
				Type:       TYPE_INT,
				Label:      "ID",
				Required:   true,
				ReadOnly:   true,
				Mode:       sysMode,
				Options:    map[string]interface{}{"primary": true, "auto": true},
				Validation: map[string]interface{}{"min": 1},
			},
			{
				Name:     "uuid",
				Type:     TYPE_STRING,
				Label:    "UUID",
				Required: true,
				ReadOnly: true,
				Mode:     sysMode,
				Options:  map[string]interface{}{"unique": true, "auto": true},
			},
			{
				Name:     "created",
				Type:     TYPE_DATE_TIME,
				Label:    "Created",
				Required: true,
				ReadOnly: true,
				Mode:     sysMode,
				Options:  map[string]interface{}{"auto": true},
			},
			{
				Name:     "updated",
				Type:     TYPE_DATE_TIME,
				Label:    "Updated",
				Required: true,
				ReadOnly: true,
				Mode:     sysMode,
				Options:  map[string]interface{}{"auto": true},
			},
			{
				Name:     "created_by",
				Type:     TYPE_INT,
				Label:    "Created By",
				Required: false,
				ReadOnly: true,
				Mode:     sysMode,
				Options:  map[string]interface{}{"auto": true},
			},
			{
				Name:         "access",
				Type:         TYPE_INT,
				Label:        "Access Level",
				Required:     false,
				ReadOnly:     false,
				Mode:         MODE_LIST | MODE_VIEW | MODE_EDIT | MODE_CREATE,
				DefaultValue: 0,
				Options:      map[string]interface{}{},
				Validation:   map[string]interface{}{"min": 0},
			},
		}
		// System fields are engine-managed. If a module declares one of the
		// non-editable system fields itself (e.g. its own uuid), force it
		// read-only so it can never be edited regardless of how it was declared.
		// access is excluded: it is the one system field admins are meant to set.
		for i := range fields {
			if IsSystemField(fields[i].Name) && fields[i].Name != "access" {
				fields[i].ReadOnly = true
			}
		}

		fieldMap := map[string]bool{}
		for _, f := range fields {
			fieldMap[f.Name] = true
		}
		// Engine-managed defaults the module opted out of (OmitSystemFields).
		omit := map[string]bool{}
		for _, name := range m.OmitSystemFields {
			omit[name] = true
		}
		for i := len(defaultFields) - 1; i >= 0; i-- {
			df := defaultFields[i]
			if omit[df.Name] {
				continue
			}
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
	ModuleLog.Debugf("Creating controller wrapper for module: %s", m.ID)

	moduleWrapper := &ModuleAbstract[interface{}]{
		ID:      m.ID,
		Name:    m.Name,
		Fields:  m.Fields,
		Filters: m.Filters,
		Rights:  m.Rights,
	}

	ModuleLog.Debugf("Creating base controller for module: %s", m.ID)
	m.Controller = NewBaseController(moduleWrapper, tableName)

	if m.Controller == nil {
		panic(fmt.Sprintf("failed to create controller for module %s", m.ID))
	}

	// Register globally for automatic routing
	ModuleLog.Debugf("Registering module globally: %s", m.ID)
	RegisteredModules[m.ID] = m

	// Publish menu metadata so /api/modules can list it (no go.config.json).
	registerModuleMenu(ModuleMenuMeta{ID: m.ID, Name: m.Name, Description: m.Description, Order: m.Order, Submenu: m.Submenu, Icon: m.Icon, ReadOnly: m.ReadOnly, Hidden: m.Hidden})

	// Advertise the module's CRUD root (/<id>) as a data endpoint so the
	// middleware can tell it from an SPA page without a hardcoded list.
	RegisterEndpointPrefix("/" + m.ID)
	for _, rt := range m.CustomRoutes {
		if rt.Absolute {
			RegisterEndpointPrefix(rt.Path)
		}
	}

	// Register default permission for rights system. Only promote an unset
	// default to READ; a module that explicitly declared 0 (DENY) — an admin-only
	// module — keeps it.
	if m.DefaultPermission == 0 && !m.DefaultPermissionSet {
		m.DefaultPermission = PERMISSION_READ
	}
	RegisterModuleDefaultPermission(m.ID, m.DefaultPermission)

	// Automatically register routes if GlobalRouter is available and auto-registration is enabled
	if GlobalRouter != nil && AutoRegisterRoutes {
		ModuleLog.Debugf("Auto-registering routes for module: %s", m.ID)
		registerSingleModuleRoutes(GlobalRouter, m)
	}

	if GlobalFieldsetHandler != nil {
		ModuleLog.Debugf("Registering module with fieldset handler: %s", m.ID)
		GlobalFieldsetHandler.RegisterModule(moduleWrapper)
	} else {
		ModuleLog.Warnf("GlobalFieldsetHandler is nil for module: %s", m.ID)
	}
}

// Default CRUD methods - can be overridden by setting CustomHandler
func (m *ModuleAbstract[T]) List(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	// Log module event using new logging system
	sessionID := getSessionIDFromRequest(r)
	userID := getUserIDFromRequest(r)

	log.LogModuleEvent(
		m.ID,
		"list",
		fmt.Sprintf("Module %s list operation started", m.ID),
		userID,
		sessionID,
		map[string]interface{}{
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

	if m.Overrides.List != nil {
		m.Overrides.List(w, r)
		return
	}
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

	if m.Overrides.View != nil {
		m.Overrides.View(w, r)
		return
	}
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

	if m.Overrides.Create != nil {
		m.Overrides.Create(w, r)
		return
	}
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

	if m.Overrides.Edit != nil {
		m.Overrides.Edit(w, r)
		return
	}
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

	if m.Overrides.Delete != nil {
		m.Overrides.Delete(w, r)
		return
	}
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
			query += field.GetSQL()
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
// addMissingColumns brings an existing table up to date with the fieldset by
// adding any missing stored column. It never drops or alters existing columns.
// Virtual and SQL-computed fields (e.g. an IMAGE preview aliased to another
// column) are skipped since they aren't real columns.
func (m *ModuleAbstract[T]) addMissingColumns(db *pgdb.Db) error {
	rows, err := db.RQuery(
		`SELECT column_name FROM information_schema.columns WHERE table_name = $1`, m.ID)
	if err != nil {
		return err
	}
	existing := map[string]bool{}
	for _, r := range rows {
		if c, ok := r["column_name"].(string); ok {
			existing[strings.ToLower(c)] = true
		}
	}
	for _, field := range m.Fields {
		if field.Virtual || field.SQL != "" {
			continue // not a real stored column
		}
		if existing[strings.ToLower(field.Name)] {
			continue
		}
		sqlType := m.fieldTypeToSQL(field)
		alter := fmt.Sprintf("ALTER TABLE %s ADD COLUMN IF NOT EXISTS %s %s", m.ID, field.Name, sqlType)
		if _, err := db.Query(alter); err != nil {
			return fmt.Errorf("add column %s.%s: %w", m.ID, field.Name, err)
		}
	}
	return nil
}

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
	case TYPE_INT, TYPE_BITMASK_SELECT:
		sqlType = "INTEGER"
	case TYPE_FLOAT:
		sqlType = "NUMERIC"
	case TYPE_STRING:
		sqlType = "VARCHAR(255)"
	case TYPE_TEXT, TYPE_HTML, TYPE_MARKDOWN:
		sqlType = "TEXT"
	case TYPE_DATE:
		sqlType = "DATE"
	case TYPE_DATE_TIME:
		sqlType = "TIMESTAMP WITH TIME ZONE"
	case TYPE_CHECKBOX, TYPE_YES_NO:
		sqlType = "BOOLEAN"
	case TYPE_JSON:
		sqlType = "JSONB"
	case TYPE_IMAGE:
		// An image field stores an array of image references
		// ({id, uuid, filename}); the bytes live in the images table.
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
		// Table exists — bring it up to date with the fieldset by adding any
		// newly-declared columns (never drops/alters existing ones). This means a
		// module can gain a field without a manual migration, e.g. the rights
		// modules' "fields" column that activates field-level rights.
		if err := m.addMissingColumns(db); err != nil {
			ModuleLog.Warnf("reconcile columns for %s: %v", m.ID, err)
		}
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
	ModuleLog.Debugf("Registering routes for module: %s", moduleID)

	// Create subrouter for this module
	subrouter := router.PathPrefix("/" + moduleID).Subrouter()

	// Register CRUD routes
	subrouter.HandleFunc("", module.List).Methods("GET")
	subrouter.HandleFunc("", module.Create).Methods("POST")
	subrouter.HandleFunc("/{id}", module.View).Methods("GET")
	subrouter.HandleFunc("/{id}", module.Edit).Methods("PUT", "PATCH")
	subrouter.HandleFunc("/{id}", module.Delete).Methods("DELETE")

	// Register any module-supplied routes beyond CRUD (e.g. binary upload/serve).
	// Absolute paths go on the root router; others are relative to /<moduleID>.
	if cr, ok := module.(CustomRouter); ok {
		for _, rt := range cr.GetCustomRoutes() {
			methods := rt.Methods
			if len(methods) == 0 {
				methods = []string{"GET"}
			}
			if rt.Absolute {
				router.HandleFunc(rt.Path, rt.Handler).Methods(methods...)
			} else {
				subrouter.HandleFunc(rt.Path, rt.Handler).Methods(methods...)
			}
			ModuleLog.Debugf("  custom route for %s: %v %s (absolute=%v)", moduleID, methods, rt.Path, rt.Absolute)
		}
	}

	ModuleLog.Debugf("Routes registered for module %s: GET,POST /%s, GET,PUT,PATCH,DELETE /%s/{id}",
		moduleID, moduleID, moduleID)
}

// RegisterModuleRoutes automatically registers CRUD routes for all registered modules
func RegisterModuleRoutes(router *mux.Router) {
	startTime := time.Now()
	moduleCount := len(RegisteredModules)

	ModuleLog.Debugf("Starting automatic route registration for %d modules", moduleCount)

	for _, module := range RegisteredModules {
		registerSingleModuleRoutes(router, module)
	}

	duration := time.Since(startTime).Milliseconds()
	ModuleLog.Debugf("Route registration completed in %dms. %d modules registered.", duration, moduleCount)
}

// SetGlobalRouter sets the global router for automatic route registration
func SetGlobalRouter(router *mux.Router) {
	GlobalRouter = router
	ModuleLog.Debugf("Global router set for automatic module route registration")

	// Register existing modules if any
	if len(RegisteredModules) > 0 {
		ModuleLog.Debugf("Registering %d existing modules with new global router", len(RegisteredModules))
		for _, module := range RegisteredModules {
			registerSingleModuleRoutes(router, module)
		}
	}
}

// EnableAutoRegistration enables or disables automatic route registration
func EnableAutoRegistration(enabled bool) {
	AutoRegisterRoutes = enabled
	if enabled {
		ModuleLog.Debugf("Automatic route registration ENABLED")
	} else {
		ModuleLog.Debugf("Automatic route registration DISABLED")
	}
}

// RegisterModuleByID manually registers routes for a specific module by ID
func RegisterModuleByID(router *mux.Router, moduleID string) error {
	module, exists := RegisteredModules[moduleID]
	if !exists {
		return fmt.Errorf("module '%s' not found in registry", moduleID)
	}

	registerSingleModuleRoutes(router, module)
	ModuleLog.Debugf("Manually registered routes for module: %s", moduleID)
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
