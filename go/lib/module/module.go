package module

import (
	"errors"

	"github.com/ws117z5/tls-rest/go/lib/db/pgdb"
)

const (
	right_non    = 1 << iota // No rights
	right_view   = 1 << iota // View rights
	right_edit   = 1 << iota // Edit rights
	right_delete = 1 << iota // Delete rights
	right_create = 1 << iota // Create rights
)

// Module represents a business module in your system.
type ModuleAbstract[T any] struct {
	ID     string
	Name   string
	Fields []Field
	//Filters []Field     // Field names that can be filtered in list view
	Rights map[int]int // userID/groupID -> Right 0

	Data []T
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
