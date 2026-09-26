package module

import (
	"context"
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
	PERMISSION_INHERIT = -1
	PERMISSION_DENY    = 0
	PERMISSION_READ    = 1
	PERMISSION_WRITE   = 2
)

type ModuleHandler interface {
	List(w http.ResponseWriter, r *http.Request)
	View(w http.ResponseWriter, r *http.Request)
	Create(w http.ResponseWriter, r *http.Request)
	Edit(w http.ResponseWriter, r *http.Request)
	Delete(w http.ResponseWriter, r *http.Request)
}

type HandlerOverrides struct {
	List   http.HandlerFunc
	View   http.HandlerFunc
	Create http.HandlerFunc
	Edit   http.HandlerFunc
	Delete http.HandlerFunc
}

type CustomRoute struct {
	Path     string
	Methods  []string
	Handler  http.HandlerFunc
	Absolute bool
}

type CustomRouter interface {
	GetCustomRoutes() []CustomRoute
}

// SpecialRight is a named permission beyond the standard modes — a module
// declares one to gate a custom button/endpoint; see auth.HasSpecialRight.
type SpecialRight struct {
	ID   string // stored in user_rights/user_group_rights.special_rights
	Name string // shown in the rights admin UI
}

type ModuleAbstract[T any] struct {
	ID                    string
	Name                  string
	Description           string
	Order                 int
	Submenu               string
	Icon                  string
	ReadOnly              bool
	Hidden                bool
	HiddenModes           []string // modes hidden from the menu without making the module fully read-only
	Fields                []Field
	Filters               *Filedset      // list-mode filters fieldset; nil = none declared
	SpecialRights         []SpecialRight // named permissions beyond the standard modes; see SpecialRight
	Rights                map[int]int
	Data                  []T
	DefaultPermission     int
	DefaultPermissionSet  bool     // true = DefaultPermission:0 is an explicit DENY, not "unset"
	OmitSystemFields      []string // engine-managed default fields to skip auto-injecting
	CustomHandler         ModuleHandler
	Controller            *BaseController
	Overrides             HandlerOverrides
	CustomRoutes          []CustomRoute
	OwnerScoped           bool                         // non-admins only see rows they created
	BeforeFieldset        Preprocessor                 // runs on the raw request body, before fieldset filtering
	AfterFieldset         Preprocessor                 // runs on the filtered data, just before the DB write
	RightsAffecting       bool                         // fires OnRightsChange after a successful write
	ConfigAffecting       bool                         // fires OnConfigChange after a successful write
	ListAscending         bool                         // default list order is oldest-first, not newest-first
	KeyField              string                       // column used to address a record by; defaults to "id"
	SoftDelete            bool                         // DELETE flags the row (needs a `deleted` column) instead of removing it
	VisibilityField       string                       // boolean column that also grants visibility past the access-level gate
	VisibilityUsersField  string                       // jsonb user-id sharing list; with VisibilityGroupsField, replaces the access-level gate
	VisibilityGroupsField string                       // jsonb group-id sharing list; see VisibilityUsersField
	CustomViews           map[string]map[string]string // mode -> {viewName: label}; documents frontend <mode>.<viewName>.tsx views (registry.ts discovers the files itself)
}

// OnRightsChange is invoked after a successful write to a RightsAffecting
// module. Wired to auth.BumpRightsEpoch (kept as a callback so this package
// doesn't import auth, which would be an import cycle).
var OnRightsChange func()

// OnConfigChange is invoked after a successful write to a ConfigAffecting
// module; wired to config.BumpConfigEpoch.
var OnConfigChange func()

// Preprocessor transforms a submitted data map during create/update — derive,
// normalize, inject, or strip fields before persistence.
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

// GetKeyField returns the column used to address a single record ("" means
// "id"), so the frontend knows which field to navigate/act on for modules
// keyed by something else (e.g. papers' uuid).
func (m *ModuleAbstract[T]) GetKeyField() string {
	return m.KeyField
}

// GetCustomRoutes exposes the module's extra routes (satisfies CustomRouter).
func (m *ModuleAbstract[T]) GetCustomRoutes() []CustomRoute {
	return m.CustomRoutes
}

// GetCustomViews exposes CustomViews (mode -> {viewName: label}) for the menu API.
func (m *ModuleAbstract[T]) GetCustomViews() map[string]map[string]string {
	return m.CustomViews
}

// GetConfigAffecting exposes ConfigAffecting for the menu API, so the client
// knows to reload AppConfig after a successful write to this module.
func (m *ModuleAbstract[T]) GetConfigAffecting() bool {
	return m.ConfigAffecting
}

func (m *ModuleAbstract[T]) GetName() string {
	return m.Name
}

func (m *ModuleAbstract[T]) GetFields() []Field {
	return m.Fields
}

func (m *ModuleAbstract[T]) GetSpecialRights() []SpecialRight {
	return m.SpecialRights
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
	GetFilters() *Filedset
	GetSpecialRights() []SpecialRight
	IsHidden() bool
	IsReadOnly() bool
	GetHiddenModes() []string
	GetKeyField() string
	GetCustomViews() map[string]map[string]string
	GetConfigAffecting() bool
	List(w http.ResponseWriter, r *http.Request)
	View(w http.ResponseWriter, r *http.Request)
	Create(w http.ResponseWriter, r *http.Request)
	Edit(w http.ResponseWriter, r *http.Request)
	Delete(w http.ResponseWriter, r *http.Request)
}

// Initializer is any *ModuleAbstract[T], regardless of T; only
// app.RegisterModule should call Initialize on one.
type Initializer interface {
	ModuleInterface
	Initialize(tableName string)
}

var RegisteredModules = make(map[string]ModuleInterface)
var ModuleDefaultPermissions = make(map[string]int)

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

// ModuleLog is the console-only logger for module lifecycle diagnostics
// (never file/db).
var ModuleLog = log.Console.With("module")

func RegisterModuleDefaultPermission(module string, defaultPermission int) {
	ModuleDefaultPermissions[module] = defaultPermission
	ModuleLog.Debugf("Default permission %d registered for module %s", defaultPermission, module)
}

// SetModuleDefaultPermission sets default permission for a module instance
func (m *ModuleAbstract[T]) SetDefaultPermission(permission int) {
	m.DefaultPermission = permission
	RegisterModuleDefaultPermission(m.ID, permission)
}

// LogModuleEvent logs a module event with structured data.
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

func NewModuleEventFromRequest(moduleID, action string, r *http.Request) ModuleEvent {
	event := ModuleEvent{
		Timestamp:  time.Now(),
		ModuleID:   moduleID,
		Action:     action,
		RemoteAddr: r.RemoteAddr,
		UserAgent:  r.UserAgent(),
		Success:    true,
	}
	if userID := extractUserID(r); userID != "" {
		event.UserID = userID
	}
	return event
}

// extractUserID tries to extract a user id from the Authorization header, an
// X-User-ID header, or the request context, in that order.
func extractUserID(r *http.Request) string {
	if auth := r.Header.Get("Authorization"); auth != "" {
		if strings.HasPrefix(auth, "Bearer ") {
			return "user_from_token"
		}
	}
	if userID := r.Header.Get("X-User-ID"); userID != "" {
		return userID
	}
	if ctx := r.Context(); ctx != nil {
		if userID := ctx.Value("userID"); userID != nil {
			if uid, ok := userID.(string); ok {
				return uid
			}
		}
	}
	return ""
}

// Initialize wires the module to a controller for tableName; call after the
// module's fields/config are fully declared.
func (m *ModuleAbstract[T]) Initialize(tableName string) {
	startTime := time.Now()

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

	// addDefaultFields injects the system fields (id/uuid/created/updated/
	// created_by): read-only, list/view/edit only. access is the exception —
	// editable and shown in create too, since it's the per-record access level.
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
				Type:     TYPE_UUID,
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
		for i := range fields {
			if IsSystemField(fields[i].Name) && fields[i].Name != "access" {
				fields[i].ReadOnly = true
			}
		}

		fieldMap := map[string]bool{}
		for _, f := range fields {
			fieldMap[f.Name] = true
		}
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
	m.Fields = addDefaultFields(ExpandReferences(m.Fields))

	if m == nil {
		panic("module is nil")
	}
	if m.ID == "" {
		panic("module ID is empty")
	}
	if tableName == "" {
		panic("table name is empty")
	}

	ModuleLog.Debugf("Creating controller wrapper for module: %s", m.ID)

	moduleWrapper := &ModuleAbstract[interface{}]{
		ID:      m.ID,
		Name:    m.Name,
		Fields:  m.Fields,
		Filters: m.Filters,
		Rights:  m.Rights,
		// Behaviour the BaseController / FieldsetEngine read off bc.Module /
		// fe.Module must be carried onto the wrapper or it is silently inert.
		BeforeFieldset:        m.BeforeFieldset,
		AfterFieldset:         m.AfterFieldset,
		KeyField:              m.KeyField,
		OwnerScoped:           m.OwnerScoped,
		SoftDelete:            m.SoftDelete,
		VisibilityField:       m.VisibilityField,
		VisibilityUsersField:  m.VisibilityUsersField,
		VisibilityGroupsField: m.VisibilityGroupsField,
		ListAscending:         m.ListAscending,
		RightsAffecting:       m.RightsAffecting,
		ConfigAffecting:       m.ConfigAffecting,
		OmitSystemFields:      m.OmitSystemFields,
	}

	ModuleLog.Debugf("Creating base controller for module: %s", m.ID)
	m.Controller = NewBaseController(moduleWrapper, tableName)

	if m.Controller == nil {
		panic(fmt.Sprintf("failed to create controller for module %s", m.ID))
	}

	ModuleLog.Debugf("Registering module globally: %s", m.ID)
	RegisteredModules[m.ID] = m
	moduleControllers[m.ID] = m.Controller

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

	if GlobalFieldsetHandler != nil {
		ModuleLog.Debugf("Registering module with fieldset handler: %s", m.ID)
		GlobalFieldsetHandler.RegisterModule(moduleWrapper)
	} else {
		ModuleLog.Warnf("GlobalFieldsetHandler is nil for module: %s", m.ID)
	}
}

// dispatch routes one CRUD action through the override → CustomHandler →
// default controller chain (first non-nil wins), wrapped in a single timed
// module event.
func (m *ModuleAbstract[T]) dispatch(w http.ResponseWriter, r *http.Request, action string, override http.HandlerFunc, custom, controller func(http.ResponseWriter, *http.Request)) {
	startTime := time.Now()
	event := NewModuleEventFromRequest(m.ID, action, r)
	if vars := mux.Vars(r); vars != nil {
		if id, ok := vars["id"]; ok {
			event.RecordID = id
		}
	}
	defer func() {
		event.Duration = time.Since(startTime).Milliseconds()
		LogModuleEvent(event)
	}()

	switch {
	case override != nil:
		override(w, r)
	case custom != nil:
		custom(w, r)
	case controller != nil:
		controller(w, r)
	}
}

// Default CRUD methods - can be overridden by setting CustomHandler
func (m *ModuleAbstract[T]) List(w http.ResponseWriter, r *http.Request) {
	var custom, controller func(http.ResponseWriter, *http.Request)
	if m.CustomHandler != nil {
		custom = m.CustomHandler.List
	}
	if m.Controller != nil {
		controller = m.Controller.List
	}
	m.dispatch(w, r, "LIST", m.Overrides.List, custom, controller)
}

func (m *ModuleAbstract[T]) View(w http.ResponseWriter, r *http.Request) {
	var custom, controller func(http.ResponseWriter, *http.Request)
	if m.CustomHandler != nil {
		custom = m.CustomHandler.View
	}
	if m.Controller != nil {
		controller = m.Controller.View
	}
	m.dispatch(w, r, "VIEW", m.Overrides.View, custom, controller)
}

func (m *ModuleAbstract[T]) Create(w http.ResponseWriter, r *http.Request) {
	var custom, controller func(http.ResponseWriter, *http.Request)
	if m.CustomHandler != nil {
		custom = m.CustomHandler.Create
	}
	if m.Controller != nil {
		controller = m.Controller.Create
	}
	m.dispatch(w, r, "CREATE", m.Overrides.Create, custom, controller)
}

func (m *ModuleAbstract[T]) Edit(w http.ResponseWriter, r *http.Request) {
	var custom, controller func(http.ResponseWriter, *http.Request)
	if m.CustomHandler != nil {
		custom = m.CustomHandler.Edit
	}
	if m.Controller != nil {
		controller = m.Controller.Edit
	}
	m.dispatch(w, r, "EDIT", m.Overrides.Edit, custom, controller)
}

func (m *ModuleAbstract[T]) Delete(w http.ResponseWriter, r *http.Request) {
	var custom, controller func(http.ResponseWriter, *http.Request)
	if m.CustomHandler != nil {
		custom = m.CustomHandler.Delete
	}
	if m.Controller != nil {
		controller = m.Controller.Delete
	}
	m.dispatch(w, r, "DELETE", m.Overrides.Delete, custom, controller)
}

// isStoredColumn reports whether a field is backed by a real table column
// (virtual and SQL-computed fields aren't) — except a TYPE_TABLE field whose
// rows are still written to a real JSONB column via TableOnSubmit.
func isStoredColumn(field Field) bool {
	if field.Virtual {
		return false
	}
	if field.Type == TYPE_TABLE {
		return field.TableSubmitFunc != nil
	}
	return field.SQL == ""
}

// addMissingColumns brings an existing table up to date with the fieldset,
// adding any missing stored column; it never drops or alters existing ones.
// All ALTER TABLEs run in one transaction (Postgres DDL is transactional) so
// a bad field spec can't leave the table with only some of its new columns.
func (m *ModuleAbstract[T]) addMissingColumns(db *pgdb.Db) error {
	existing, err := tableColumns(db, m.ID)
	if err != nil {
		return err
	}
	return db.WithTransaction(func(tx *pgdb.Db) error {
		for _, field := range m.Fields {
			if !isStoredColumn(field) {
				continue
			}
			if existing[strings.ToLower(field.Name)] {
				continue
			}
			sqlType := m.fieldTypeToSQL(field)
			sqlType = strings.Replace(sqlType, " NOT NULL", "", 1)
			if t, ok := timestampSystemColumnType(field.Name); ok {
				sqlType = t
			} else if field.Name == "access" {
				sqlType = "INTEGER DEFAULT 0"
			}
			alter := fmt.Sprintf("ALTER TABLE %s ADD COLUMN IF NOT EXISTS %s %s", m.ID, field.Name, sqlType)
			if _, err := tx.Query(alter); err != nil {
				return fmt.Errorf("add column %s.%s: %w", m.ID, field.Name, err)
			}
		}
		return nil
	})
}

func (m *ModuleAbstract[T]) getDB(ctx context.Context) (*pgdb.Db, error) {
	return pgdb.GetInstanceCtx(ctx)
}

var modules = make(map[string]*ModuleAbstract[any])

// CreateModule creates a new module.
func CreateModule[T any](id, name string) (*ModuleAbstract[T], error) {
	if _, exists := modules[id]; exists {
		return nil, errors.New("module already exists")
	}
	m := &ModuleAbstract[T]{
		ID:     id,
		Name:   name,
		Fields: []Field{},
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

// timestampSystemColumnType returns the column type the engine forces for a
// timestamp system field (created/updated and their _at variants), and whether
// name is one. Shared by CREATE TABLE generation and column reconciliation so
// both emit the same "DEFAULT now()" definition.
func timestampSystemColumnType(name string) (string, bool) {
	switch name {
	case "created", "created_at", "updated", "updated_at":
		return "TIMESTAMP WITH TIME ZONE DEFAULT now()", true
	}
	return "", false
}

func (m *ModuleAbstract[T]) fieldTypeToSQL(field Field) string {
	var sqlType string

	switch field.Type {
	case TYPE_INT, TYPE_BITMASK_SELECT:
		sqlType = "INTEGER"
	case TYPE_FLOAT:
		sqlType = "NUMERIC"
	case TYPE_STRING:
		sqlType = "VARCHAR(255)"
	case TYPE_UUID:
		sqlType = "UUID"
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
		// A table field's rows are stored as JSON on the record (or, for a
		// TableSource-backed table, kept out of the fieldset's own SELECT and
		// loaded via the /table/{field} endpoint).
		sqlType = "JSONB"
	case TYPE_MONEY:
		sqlType = "NUMERIC(15,2)"
	default:
		sqlType = "TEXT"
	}

	if field.Required {
		sqlType += " NOT NULL"
	}

	// A TYPE_TABLE field's SQL is a SELECT projection for display, not a
	// column default, so it's excluded from the "custom SQL as default" case.
	if field.SQL != "" && field.Type != TYPE_TABLE {
		sqlType += fmt.Sprintf(" DEFAULT %s", field.SQL)
	} else if field.DefaultValue != nil {
		switch v := field.DefaultValue.(type) {
		case string:
			sqlType += fmt.Sprintf(" DEFAULT '%s'", v)
		case int, int64, float64:
			sqlType += fmt.Sprintf(" DEFAULT %v", v)
		case bool:
			sqlType += fmt.Sprintf(" DEFAULT %t", v)
		}
	} else if field.Type == TYPE_UUID {
		// Every auto-created UUID column generates its own value, so inserts that
		// omit it (and columns reconciled onto an existing table) still get one.
		sqlType += " DEFAULT uuid_generate_v4()"
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

	db, err := m.getDB(context.Background())
	if err != nil {
		event.Success = false
		event.Error = fmt.Sprintf("failed to get database connection: %v", err)
		return fmt.Errorf("failed to get database connection: %w", err)
	}

	exists, err := db.TableExists(m.ID)
	if err != nil {
		event.Success = false
		event.Error = fmt.Sprintf("failed to check if table exists: %v", err)
		return fmt.Errorf("failed to check if table exists: %w", err)
	}

	if exists {
		// Lets a module gain a field without a manual migration, e.g. the
		// rights modules' "fields" column that activates field-level rights.
		if err := m.addMissingColumns(db); err != nil {
			ModuleLog.Warnf("reconcile columns for %s: %v", m.ID, err)
		}
		event.Details = "Table already exists"
		return nil
	}

	event.Action = "TABLE_CREATE"
	event.Details = fmt.Sprintf("Creating table with %d fields", len(m.Fields))

	// The sequence and the table that defaults its id column to it are one
	// atomic unit — either both exist afterward or neither does.
	if txErr := db.WithTransaction(func(tx *pgdb.Db) error {
		for _, field := range m.Fields {
			if field.Name == "id" && field.Type == TYPE_INT {
				sequenceSQL := fmt.Sprintf(`CREATE SEQUENCE IF NOT EXISTS %s_id_seq`, m.ID)
				if _, err := tx.Query(sequenceSQL); err != nil {
					return fmt.Errorf("failed to create sequence for table %s: %w", m.ID, err)
				}
				break
			}
		}
		if _, err := tx.Query(m.generateCreateTableSQL()); err != nil {
			return fmt.Errorf("failed to create table %s: %w", m.ID, err)
		}
		return nil
	}); txErr != nil {
		event.Success = false
		event.Error = txErr.Error()
		return txErr
	}

	event.Details += " - SUCCESS"
	return nil
}

// generateCreateTableSQL generates the CREATE TABLE SQL statement
func (m *ModuleAbstract[T]) generateCreateTableSQL() string {
	var columns []string
	var primaryKeys []string

	for _, field := range m.Fields {
		if !isStoredColumn(field) {
			continue
		}

		sqlType := m.fieldTypeToSQL(field)
		if sqlType == "" {
			continue
		}

		columnDef := fmt.Sprintf("%s %s", field.Name, sqlType)

		switch field.Name {
		case "id":
			if field.Type == TYPE_INT {
				columnDef = fmt.Sprintf("id INTEGER NOT NULL DEFAULT nextval('%s_id_seq'::regclass)", m.ID)
			}
			primaryKeys = append(primaryKeys, field.Name)
		case "uuid":
			if field.Type == TYPE_UUID || (field.Type == TYPE_STRING && field.SQL != "") {
				columnDef = "uuid UUID NOT NULL DEFAULT uuid_generate_v4()"
			}
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
		case "created", "created_at", "updated", "updated_at":
			if t, ok := timestampSystemColumnType(field.Name); ok && field.Type == TYPE_DATE_TIME {
				columnDef = field.Name + " " + t
			}
		}

		columns = append(columns, columnDef)
	}

	sql := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n    %s", m.ID, strings.Join(columns, ",\n    "))

	if len(primaryKeys) > 0 {
		sql += fmt.Sprintf(",\n    CONSTRAINT %s_pkey PRIMARY KEY (%s)", m.ID, strings.Join(primaryKeys, ", "))
	}

	sql += "\n)"

	return sql
}

func (m *ModuleAbstract[T]) GenerateCreateTableSQL() string {
	return m.generateCreateTableSQL()
}

func (m *ModuleAbstract[T]) FieldTypeToSQL(field Field) string {
	return m.fieldTypeToSQL(field)
}

// GetRegisteredModuleIDs returns a list of all registered module IDs
func GetRegisteredModuleIDs() []string {
	ids := make([]string, 0, len(RegisteredModules))
	for id := range RegisteredModules {
		ids = append(ids, id)
	}
	return ids
}
