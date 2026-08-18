package module

import "fmt"

const MODE_LIST = 0b000001
const MODE_VIEW = 0b000010
const MODE_EDIT = 0b000100
const MODE_LOG = 0b001000
const MODE_MULTIPLEUPDATE = 0b010000
const MODE_SUBMIT = 0b100000
const MODE_CREATE = 0b1000000
const MODE_DELETE = 0b10000000
const MODE_READONLY = 0b000011
const MODE_EDITSUBMIT = 0b100100 // MODE_EDIT + MODE_SUBMIT
const MODE_ALL = 0b11111111

const TYPE_HTML = "Html"

const TYPE_CHECKBOX = "Checkbox"
const TYPE_CHECKBOX_SET = "CheckboxSet"
const TYPE_CHECKBOX_AJAX = "CheckboxAjax"

const TYPE_STRING = "String"
const TYPE_AUTOCOMPLETE = "Autocomplete"

const TYPE_TEXT = "Text"
const TYPE_MARKDOWN = "Markdown"
const TYPE_JSON = "Json"
const TYPE_IMAGE = "Image"
const TYPE_AUTOCOMPLETE_TEXT = "AutocompleteText"

const TYPE_DATE = "Date"
const TYPE_DATE_TIME = "DateTime"
const TYPE_COLOR = "Color"
const TYPE_WEEK = "Week"

const TYPE_INT = "Int"
const TYPE_FLOAT = "Float"
const TYPE_MONEY = "Money"
const TYPE_MONEY_WITH_CURRENCY = "MoneyWithCurrency"

const TYPE_SELECT = "Select"
const TYPE_SELECT_ADDNEW = "SelectAddNew"
const TYPE_SELECT_CHILD = "SelectChild"
const TYPE_SELECT2_MULTIPLE = "Select2Multiple"
const TYPE_ACTIVE_INACTIVE = "ActiveInactive"
const TYPE_YES_NO = "YesNo"
const TYPE_MONTH = "Month"
const TYPE_TABLE = "Table"

// Field represents a field in a module.
type Field struct {
	Name         string                 `json:"name"`
	Type         string                 `json:"type"`
	Required     bool                   `json:"required"`
	SQL          string                 `json:"sql"`           // SQL column definition or expression
	SQLWhere     string                 `json:"sql_where"`     // Custom SQL WHERE clause template
	Filterable   bool                   `json:"filterable"`    // Whether this field can be used for filtering
	Sortable     bool                   `json:"sortable"`      // Whether this field can be used for sorting
	Searchable   bool                   `json:"searchable"`    // Whether this field is included in search
	Virtual      bool                   `json:"virtual"`       // Whether this field is virtual (not stored in DB)
	ReadOnly     bool                   `json:"readonly"`      // Whether this field is read-only
	DefaultValue interface{}            `json:"default_value"` // Default value for the field
	Validation   map[string]interface{} `json:"validation"`    // Validation rules
	Options      map[string]interface{} `json:"options"`       // Field-specific options (autocomplete URL, etc.)
	Mode         int                    `json:"mode"`          // Bit flags for which modes this field appears in
	Label        string                 `json:"label"`         // Display label
	Description  string                 `json:"description"`   // Field description
	Access       int                    `json:"access"`        // Minimum access level required to see this field (0 = everyone)
}

func NewField(name, fieldType string, required bool) Field {
	ret := Field{
		Name:       name,
		Type:       fieldType,
		Required:   required,
		Label:      name,                    // Default label is the field name
		Filterable: fieldType != TYPE_TABLE, // Tables are not filterable by default
		Sortable:   fieldType != TYPE_TABLE, // Tables are not sortable by default
		Searchable: fieldType == TYPE_STRING || fieldType == TYPE_TEXT || fieldType == TYPE_AUTOCOMPLETE,
		Mode:       MODE_ALL, // Default to all modes
		Validation: make(map[string]interface{}),
		Options:    make(map[string]interface{}),
	}

	return ret
}

// Field builder methods for fluent configuration
func (f Field) WithSQL(sql string) Field {
	f.SQL = sql
	return f
}

func (f Field) WithSQLWhere(where string) Field {
	f.SQLWhere = where
	return f
}

func (f Field) WithLabel(label string) Field {
	f.Label = label
	return f
}

func (f Field) WithDescription(desc string) Field {
	f.Description = desc
	return f
}

func (f Field) WithDefault(value interface{}) Field {
	f.DefaultValue = value
	return f
}

func (f Field) WithMode(mode int) Field {
	f.Mode = mode
	return f
}

// WithAccess sets the minimum access level a user must have to see this field.
// 0 (the default) means everyone; higher values hide the field from lower-level
// users. Enforced at request time by the fieldset engine.
func (f Field) WithAccess(level int) Field {
	f.Access = level
	return f
}

func (f Field) WithValidation(key string, value interface{}) Field {
	f.Validation[key] = value
	return f
}

func (f Field) WithOption(key string, value interface{}) Field {
	f.Options[key] = value
	return f
}

// WithoutDescription drops this field's description column in view/edit forms, so
// the value spans the full row.
func (f Field) WithoutDescription() Field {
	f.Options["hideDescription"] = true
	return f
}

// WithDescriptionWidth predefines this field's description-column width (px) for
// view/edit forms. The whole form's description column takes the widest such
// value across its fields.
func (f Field) WithDescriptionWidth(px int) Field {
	f.Options["descriptionWidth"] = px
	return f
}

// WithValueWidth predefines this field's value-column width (px) for view/edit
// forms. The whole form's value column takes the widest such value across its
// fields (unset => the value column flex-fills the row).
func (f Field) WithValueWidth(px int) Field {
	f.Options["valueWidth"] = px
	return f
}

func (f Field) NonFilterable() Field {
	f.Filterable = false
	return f
}

func (f Field) NonSortable() Field {
	f.Sortable = false
	return f
}

func (f Field) NonSearchable() Field {
	f.Searchable = false
	return f
}

func (f Field) AsVirtual() Field {
	f.Virtual = true
	return f
}

func (f Field) AsReadOnly() Field {
	f.ReadOnly = true
	return f
}

func (f Field) WithDefaultValue(value interface{}) Field {
	f.DefaultValue = value
	return f
}

// Table-specific configuration methods
func (f Field) WithTableColumns(columns []string) Field {
	if f.Type != TYPE_TABLE {
		return f
	}
	if f.Options == nil {
		f.Options = make(map[string]interface{})
	}
	f.Options["columns"] = columns
	return f
}

func (f Field) WithTableData(data interface{}) Field {
	if f.Type != TYPE_TABLE {
		return f
	}
	if f.Options == nil {
		f.Options = make(map[string]interface{})
	}
	f.Options["data"] = data
	f.Options["dataSource"] = "static"
	return f
}

// Configure table to fetch data from database table
func (f Field) WithDatabaseTable(tableName string) Field {
	if f.Type != TYPE_TABLE {
		return f
	}
	if f.Options == nil {
		f.Options = make(map[string]interface{})
	}
	f.Options["sourceTable"] = tableName
	f.Options["dataSource"] = "database"
	return f
}

// Configure custom SQL query for table data
func (f Field) WithTableQuery(query string) Field {
	if f.Type != TYPE_TABLE {
		return f
	}
	if f.Options == nil {
		f.Options = make(map[string]interface{})
	}
	f.Options["query"] = query
	f.Options["dataSource"] = "query"
	return f
}

// Configure table with query parameters
func (f Field) WithTableQueryParams(params map[string]interface{}) Field {
	if f.Type != TYPE_TABLE {
		return f
	}
	if f.Options == nil {
		f.Options = make(map[string]interface{})
	}
	f.Options["queryParams"] = params
	return f
}

// Configure foreign key relationship for table updates
func (f Field) WithTableForeignKey(foreignKeyColumn, parentColumn string) Field {
	if f.Type != TYPE_TABLE {
		return f
	}
	if f.Options == nil {
		f.Options = make(map[string]interface{})
	}
	f.Options["foreignKey"] = map[string]string{
		"column":       foreignKeyColumn,
		"parentColumn": parentColumn,
	}
	return f
}

func (f Field) WithTableSubmitFunction(submitFunc string) Field {
	if f.Type != TYPE_TABLE {
		return f
	}
	if f.Options == nil {
		f.Options = make(map[string]interface{})
	}
	f.Options["submitFunction"] = submitFunc
	// If submit function is provided, mark field as editable
	f.Options["editable"] = true
	return f
}

func (f Field) WithTableEditable(editable bool) Field {
	if f.Type != TYPE_TABLE {
		return f
	}
	if f.Options == nil {
		f.Options = make(map[string]interface{})
	}
	f.Options["editable"] = editable

	// If editable is true but no submit function is provided, require one
	if editable {
		if _, hasSubmitFunc := f.Options["submitFunction"]; !hasSubmitFunc {
			// This will be validated later
			f.Options["requiresSubmitFunction"] = true
		}
	}
	return f
}

func (f Field) WithTableRowActions(actions []map[string]interface{}) Field {
	if f.Type != TYPE_TABLE {
		return f
	}
	if f.Options == nil {
		f.Options = make(map[string]interface{})
	}
	f.Options["rowActions"] = actions
	return f
}

// ValidateTableField validates table field configuration
func (f Field) ValidateTableField() error {
	if f.Type != TYPE_TABLE {
		return nil
	}

	if f.Options == nil {
		return fmt.Errorf("table field %s must have options configured", f.Name)
	}

	// Validate data source configuration
	dataSource, ok := f.Options["dataSource"].(string)
	if !ok {
		dataSource = "static"
	}

	switch dataSource {
	case "database":
		if _, hasSourceTable := f.Options["sourceTable"]; !hasSourceTable {
			return fmt.Errorf("database table field %s requires sourceTable configuration", f.Name)
		}
	case "query":
		if _, hasQuery := f.Options["query"]; !hasQuery {
			return fmt.Errorf("query table field %s requires query configuration", f.Name)
		}
	case "static":
		// Static data is optional, can be set later
	default:
		return fmt.Errorf("invalid dataSource %s for table field %s", dataSource, f.Name)
	}

	// Check if editable is true but no submit function
	if editable, ok := f.Options["editable"].(bool); ok && editable {
		if _, hasSubmitFunc := f.Options["submitFunction"]; !hasSubmitFunc {
			return fmt.Errorf("editable table field %s requires a submit function", f.Name)
		}
	}

	return nil
}

type Filedset struct {
	Fields []Field
}

func NewFieldset(fields ...Field) *Filedset {
	return &Filedset{
		Fields: fields,
	}
}

func (f *Field) getSQL() string {
	if f.SQL != "" {
		return f.SQL
	}
	return f.Name
}

func (f *Field) getSQLWhere() string {
	if f.SQLWhere != "" {
		return f.SQLWhere
	}
	return f.Name + " = ?"
}
