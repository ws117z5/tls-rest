package module

import (
	"strings"
	"testing"
)

// TestTableFieldType tests the new TABLE field type functionality
func TestTableFieldType(t *testing.T) {
	// Test creating a basic table field
	tableField := NewField("user_roles", TYPE_TABLE, false).
		WithLabel("User Roles").
		WithDescription("Manage user roles and permissions").
		WithTableColumns([]string{"role", "permission", "granted_at"})

	// Verify basic properties
	if tableField.Type != TYPE_TABLE {
		t.Errorf("Expected field type %s, got %s", TYPE_TABLE, tableField.Type)
	}

	if tableField.Filterable {
		t.Error("Table fields should not be filterable by default")
	}

	if tableField.Sortable {
		t.Error("Table fields should not be sortable by default")
	}

	// Test table field with data
	sampleData := []map[string]interface{}{
		{"role": "admin", "permission": "read", "granted_at": "2023-01-01"},
		{"role": "user", "permission": "write", "granted_at": "2023-01-02"},
	}

	tableFieldWithData := NewField("permissions_table", TYPE_TABLE, false).
		WithLabel("Permissions").
		WithTableColumns([]string{"role", "permission", "granted_at"}).
		WithTableData(sampleData)

	// Verify data is stored in options
	if data, ok := tableFieldWithData.Options["data"]; !ok {
		t.Error("Table data should be stored in options")
	} else {
		if dataSlice, ok := data.([]map[string]interface{}); !ok {
			t.Error("Table data should be a slice of maps")
		} else if len(dataSlice) != 2 {
			t.Errorf("Expected 2 data rows, got %d", len(dataSlice))
		}
	}

	t.Logf("Basic table field created successfully: %+v", tableField)
}

// TestEditableTableField tests editable table fields with submit function
func TestEditableTableField(t *testing.T) {
	// Test editable table field with submit function
	editableTableField := NewField("editable_users", TYPE_TABLE, false).
		WithLabel("Editable Users").
		WithTableColumns([]string{"name", "email", "active"}).
		WithTableEditable(true).
		WithTableSubmitFunction("submitUserChanges")

	// Verify editable configuration
	if editable, ok := editableTableField.Options["editable"].(bool); !ok || !editable {
		t.Error("Table field should be marked as editable")
	}

	if submitFunc, ok := editableTableField.Options["submitFunction"].(string); !ok || submitFunc != "submitUserChanges" {
		t.Error("Table field should have submit function configured")
	}

	// Test validation passes for properly configured editable field
	err := editableTableField.ValidateTableField()
	if err != nil {
		t.Errorf("Validation should pass for properly configured editable table field: %v", err)
	}

	t.Logf("Editable table field created successfully: %+v", editableTableField.Options)
}

// TestEditableTableFieldValidation tests validation for editable table fields
func TestEditableTableFieldValidation(t *testing.T) {
	// Test editable table field WITHOUT submit function (should fail validation)
	invalidEditableField := NewField("invalid_editable", TYPE_TABLE, false).
		WithLabel("Invalid Editable Table").
		WithTableColumns([]string{"col1", "col2"}).
		WithTableEditable(true)
	// Note: No submit function provided

	// This should fail validation
	err := invalidEditableField.ValidateTableField()
	if err == nil {
		t.Error("Validation should fail for editable table field without submit function")
	}

	// Test non-editable table field (should pass validation even without submit function)
	validReadOnlyField := NewField("readonly_table", TYPE_TABLE, false).
		WithLabel("Read Only Table").
		WithTableColumns([]string{"col1", "col2"})
	// Note: Not marked as editable, no submit function needed

	err = validReadOnlyField.ValidateTableField()
	if err != nil {
		t.Errorf("Validation should pass for non-editable table field: %v", err)
	}

	t.Log("Table field validation tests passed")
}

// TestTableFieldWithActions tests table fields with row actions
func TestTableFieldWithActions(t *testing.T) {
	actions := []map[string]interface{}{
		{
			"label":    "Edit",
			"action":   "editRow",
			"icon":     "pencil",
			"disabled": false,
		},
		{
			"label":    "Delete",
			"action":   "deleteRow",
			"icon":     "trash",
			"disabled": false,
		},
	}

	tableFieldWithActions := NewField("users_with_actions", TYPE_TABLE, false).
		WithLabel("Users Management").
		WithTableColumns([]string{"id", "name", "email", "status"}).
		WithTableRowActions(actions).
		WithTableSubmitFunction("handleUserAction")

	// Verify actions are stored
	if storedActions, ok := tableFieldWithActions.Options["rowActions"]; !ok {
		t.Error("Row actions should be stored in options")
	} else {
		if actionSlice, ok := storedActions.([]map[string]interface{}); !ok {
			t.Error("Row actions should be a slice of maps")
		} else if len(actionSlice) != 2 {
			t.Errorf("Expected 2 actions, got %d", len(actionSlice))
		}
	}

	t.Logf("Table field with actions created successfully")
}

// TestTableFieldInModule tests using table fields in a complete module
func TestTableFieldInModule(t *testing.T) {
	// Create a module that includes a table field
	userManagementModule := &ModuleAbstract[interface{}]{
		ID:   "user_management",
		Name: "User Management",
		Fields: []Field{
			NewField("id", TYPE_INT, false).
				WithLabel("ID").
				AsReadOnly(),

			NewField("title", TYPE_STRING, true).
				WithLabel("Title").
				WithValidation("minLength", 3),

			NewField("user_permissions", TYPE_TABLE, false).
				WithLabel("User Permissions").
				WithDescription("Manage individual user permissions").
				WithTableColumns([]string{"user_id", "permission", "granted_by", "granted_at"}).
				WithTableEditable(true).
				WithTableSubmitFunction("updateUserPermissions").
				WithTableRowActions([]map[string]interface{}{
					{"label": "Revoke", "action": "revokePermission", "icon": "ban"},
				}),

			NewField("audit_log", TYPE_TABLE, false).
				WithLabel("Audit Log").
				WithDescription("View permission change history").
				WithTableColumns([]string{"timestamp", "action", "user", "details"}).
				AsReadOnly(), // Read-only table, no submit function needed

			NewField("created_at", TYPE_DATE_TIME, false).
				WithLabel("Created At").
				AsReadOnly(),
		},
		Rights: make(map[int]int),
	}

	// Validate all table fields in the module
	for _, field := range userManagementModule.Fields {
		if field.Type == TYPE_TABLE {
			err := field.ValidateTableField()
			if err != nil {
				t.Errorf("Table field %s validation failed: %v", field.Name, err)
			}
		}
	}

	// Test SQL generation for module with table fields
	sql := userManagementModule.generateCreateTableSQL()
	if sql == "" {
		t.Error("Should generate SQL even with table fields")
	}

	// Verify table fields are stored as JSONB
	if !contains(sql, "user_permissions JSONB") {
		t.Error("Table field should be stored as JSONB in database")
	}

	t.Logf("Module with table fields created successfully")
	t.Logf("Generated SQL: %s", sql)
}

// TestDatabaseTableField tests table fields that reference actual database tables
func TestDatabaseTableField(t *testing.T) {
	// Test database table field with foreign key
	userRolesField := NewField("user_roles", TYPE_TABLE, false).
		WithLabel("User Roles").
		WithDatabaseTable("user_roles").
		WithTableColumns([]string{"role_name", "assigned_at", "assigned_by"}).
		WithTableForeignKey("user_id", "id")

	// Validate configuration
	err := userRolesField.ValidateTableField()
	if err != nil {
		t.Errorf("Database table field validation failed: %v", err)
	}

	// Check data source is set to database
	if dataSource, ok := userRolesField.Options["dataSource"].(string); !ok || dataSource != "database" {
		t.Error("Database table field should have dataSource set to 'database'")
	}

	// Check source table is configured
	if sourceTable, ok := userRolesField.Options["sourceTable"].(string); !ok || sourceTable != "user_roles" {
		t.Error("Database table field should have sourceTable configured")
	}

	t.Logf("Database table field created successfully: %+v", userRolesField.Options)
}

// TestQueryTableField tests table fields that use custom SQL queries
func TestQueryTableField(t *testing.T) {
	// Test custom query table field
	statisticsField := NewField("user_statistics", TYPE_TABLE, false).
		WithLabel("User Statistics").
		WithTableQuery("SELECT action, COUNT(*) as count FROM user_actions WHERE user_id = $1 GROUP BY action").
		WithTableColumns([]string{"action", "count"}).
		WithTableQueryParams(map[string]interface{}{
			"active": true,
		})

	// Validate configuration
	err := statisticsField.ValidateTableField()
	if err != nil {
		t.Errorf("Query table field validation failed: %v", err)
	}

	// Check data source is set to query
	if dataSource, ok := statisticsField.Options["dataSource"].(string); !ok || dataSource != "query" {
		t.Error("Query table field should have dataSource set to 'query'")
	}

	// Check query is configured
	if query, ok := statisticsField.Options["query"].(string); !ok || query == "" {
		t.Error("Query table field should have query configured")
	}

	t.Logf("Query table field created successfully: %+v", statisticsField.Options)
}

// TestModuleWithDatabaseTableFields tests a complete module with database table fields
func TestModuleWithDatabaseTableFields(t *testing.T) {
	// Create a module that references actual database tables
	projectModule := &ModuleAbstract[interface{}]{
		ID:   "projects",
		Name: "Project Management",
		Fields: []Field{
			NewField("id", TYPE_INT, false).
				WithLabel("ID").
				AsReadOnly(),

			NewField("name", TYPE_STRING, true).
				WithLabel("Project Name").
				WithValidation("minLength", 3),

			// Reference to project_members table
			NewField("members", TYPE_TABLE, false).
				WithLabel("Project Members").
				WithDatabaseTable("project_members").
				WithTableColumns([]string{"user_name", "role", "joined_at"}).
				WithTableForeignKey("project_id", "id").
				WithTableEditable(true).
				WithTableSubmitFunction("updateProjectMembers"),

			// Reference to project_tasks with custom query
			NewField("task_summary", TYPE_TABLE, false).
				WithLabel("Task Summary").
				WithTableQuery(`
					SELECT status, COUNT(*) as count, 
					       AVG(EXTRACT(EPOCH FROM (completed_at - created_at))/86400) as avg_days
					FROM project_tasks 
					WHERE project_id = $1 
					GROUP BY status
				`).
				WithTableColumns([]string{"status", "count", "avg_days"}),

			// Static data table for project categories
			NewField("categories", TYPE_TABLE, false).
				WithLabel("Available Categories").
				WithTableData([]map[string]interface{}{
					{"name": "Development", "color": "#007acc"},
					{"name": "Design", "color": "#ff6b35"},
					{"name": "Marketing", "color": "#4ecdc4"},
				}).
				WithTableColumns([]string{"name", "color"}),

			NewField("created_at", TYPE_DATE_TIME, false).
				WithLabel("Created At").
				AsReadOnly(),
		},
		Rights: make(map[int]int),
	}

	// Validate all table fields
	for _, field := range projectModule.Fields {
		if field.Type == TYPE_TABLE {
			err := field.ValidateTableField()
			if err != nil {
				t.Errorf("Table field %s validation failed: %v", field.Name, err)
			}
		}
	}

	// Test SQL generation - virtual table fields should not appear in SQL
	sql := projectModule.generateCreateTableSQL()

	// Database table fields should not be in the SQL (they're virtual)
	if contains(sql, "members JSONB") {
		t.Error("Database table fields should not appear in SQL (they're virtual)")
	}

	// Static table fields should be in the SQL
	if !contains(sql, "categories JSONB") {
		t.Error("Static table fields should appear in SQL as JSONB")
	}

	t.Logf("Project module with mixed table field types created successfully")
	t.Logf("Generated SQL: %s", sql)
}

// Helper function to check if string contains substring
func contains(str, substr string) bool {
	return len(str) >= len(substr) && (str == substr ||
		len(str) > len(substr) && (str[:len(substr)] == substr ||
			str[len(str)-len(substr):] == substr ||
			strings.Contains(str, substr)))
}
