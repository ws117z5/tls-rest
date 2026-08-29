package field

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

// TYPE_BITMASK_SELECT renders a checkbox-per-option editor whose value is the
// integer OR of the selected option values (a bitmask). Options come from a
// WithOptions(func) provider.
const TYPE_BITMASK_SELECT = "BitmaskSelect"

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
	Placeholder  string                 `json:"placeholder"`   // Input placeholder (edit/create/filters)
	Access       int                    `json:"access"`        // Minimum access level required to see this field (0 = everyone)

	// Display / formatting modifiers (list & view rendering).
	LinkModule    string `json:"linkModule,omitempty"`    // render the value as a link into this module's record
	Unit          string `json:"unit,omitempty"`          // unit suffix shown after the value (e.g. "kg", "USD")
	ZeroEmpty     bool   `json:"zeroEmpty,omitempty"`     // render 0 as blank
	NegativeClass string `json:"negativeClass,omitempty"` // CSS class applied when the value is negative
	PositiveClass string `json:"positiveClass,omitempty"` // CSS class applied when the value is positive
	Align         string `json:"align,omitempty"`         // list column alignment: left|center|right
	ColumnWidth   string `json:"columnWidth,omitempty"`   // list column width (e.g. "120px")

	// TYPE_TABLE configuration (clean API — see TableFieldset/TableSource/
	// TableData/TableOnSubmit builders). The fieldset (columns) is serialized to
	// the client; the data/submit hooks are server-side only.
	TableColumns    []Field                                                   `json:"tableFieldset,omitempty"` // column definitions (each a Field)
	TableSourceName string                                                    `json:"tableSource,omitempty"`   // DB table with module_id,row_id to load rows from
	TableDataFunc   func(ctx map[string]interface{}) []map[string]interface{} `json:"-"`                       // manual row provider (wins over TableSource)
	TableSubmitFunc func(data []map[string]interface{}) interface{}           `json:"-"`                       // process submitted rows before storing

	// OptionsFunc supplies the field's options at request time (like the legacy
	// framework's dynamic 'options'). Its result is resolved into Options["options"]
	// when the fieldset is served. Set via WithOptions.
	OptionsFunc func() []map[string]interface{} `json:"-"`

	// Autocomplete configuration (set via WithAutocomplete). The kind is
	// serialized so the client renders the autocomplete widget and calls the
	// /autocomplete endpoint; the resolution (func/sql/source) is server-side.
	AutocompleteKind   string                      `json:"autocomplete,omitempty"` // "function" | "sql" | "source"
	AutocompleteFunc   func(input string) []string `json:"-"`
	AutocompleteSQL    string                      `json:"-"`
	AutocompleteSource []string                    `json:"-"` // {table, field, match}
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

// WithPlaceholder sets the input placeholder shown in edit/create forms and
// filter inputs for text-like fields.
func (f Field) WithPlaceholder(p string) Field {
	f.Placeholder = p
	return f
}

// --- Display / formatting builders ------------------------------------------

// WithLink renders the value in list/view as a link into the given module's
// record (foreign-key navigation).
func (f Field) WithLink(module string) Field {
	f.LinkModule = module
	return f
}

// WithUnit shows a unit suffix after the value (e.g. "kg", "USD").
func (f Field) WithUnit(unit string) Field {
	f.Unit = unit
	return f
}

// WithZeroEmpty renders a zero value as blank.
func (f Field) WithZeroEmpty() Field {
	f.ZeroEmpty = true
	return f
}

// WithSignClasses applies CSS classes based on the sign of a numeric value.
func (f Field) WithSignClasses(negative, positive string) Field {
	f.NegativeClass = negative
	f.PositiveClass = positive
	return f
}

// WithAlign sets list column alignment: "left", "center", or "right".
func (f Field) WithAlign(align string) Field {
	f.Align = align
	return f
}

// WithColumnWidth sets the list column width (e.g. "120px").
func (f Field) WithColumnWidth(width string) Field {
	f.ColumnWidth = width
	return f
}

// --- Mode builders ----------------------------------------------------------

// InModes sets exactly which modes the field appears in (overwrites Mode).
func (f Field) InModes(mode int) Field {
	f.Mode = mode
	return f
}

// NotSubmitted marks the field display-only: shown but never written on save
// (clears MODE_SUBMIT). Use for computed/derived columns.
func (f Field) NotSubmitted() Field {
	f.Mode &^= MODE_SUBMIT
	return f
}

// NotLogged excludes the field from the change log (clears MODE_LOG).
func (f Field) NotLogged() Field {
	f.Mode &^= MODE_LOG
	return f
}

// WithAutocomplete enables type-ahead suggestions on a (string) field, resolved
// server-side by the /api/modules/{module}/autocomplete/{field} endpoint. Three
// kinds:
//
//	WithAutocomplete("function", func(input string) []string { ... })
//	WithAutocomplete("sql", "SELECT name FROM city WHERE name LIKE $1")
//	WithAutocomplete("source", []string{"city", "name", "left"}) // table, field, match: left|right|full
func (f Field) WithAutocomplete(kind string, arg interface{}) Field {
	f.AutocompleteKind = kind
	switch kind {
	case "function":
		if fn, ok := arg.(func(string) []string); ok {
			f.AutocompleteFunc = fn
		}
	case "sql":
		if q, ok := arg.(string); ok {
			f.AutocompleteSQL = q
		}
	case "source":
		if s, ok := arg.([]string); ok {
			f.AutocompleteSource = s
		}
	}
	return f
}

// WithOptions sets a provider for the field's options, resolved at request time
// (mirrors the legacy framework's dynamic 'options'). Cleaner than stashing a
// static list under Options["bits"]/["options"]. Used by TYPE_BITMASK_SELECT and
// any select-style field:
//
//	NewField("modes", TYPE_BITMASK_SELECT, true).WithOptions(modeBitOptions)
func (f Field) WithOptions(fn func() []map[string]interface{}) Field {
	f.OptionsFunc = fn
	return f
}

// --- TYPE_TABLE clean configuration API -------------------------------------
//
// A table field is configured like a mini-module:
//
//	NewField("fields", TYPE_TABLE, false).
//	    TableFieldset([]Field{                       // the columns
//	        NewField("field", TYPE_STRING, false).AsReadOnly(),
//	        NewField("view", TYPE_CHECKBOX, false),
//	        NewField("edit", TYPE_CHECKBOX, false),
//	    }).
//	    TableSource("module_field_rights").          // OR
//	    TableData(func(ctx map[string]interface{}) []map[string]interface{} { ... }).
//	    TableOnSubmit(func(rows []map[string]interface{}) interface{} { ... })

// TableFieldset sets the table's columns as a fieldset, so each column is a real
// Field (checkbox, read-only string, etc.) that the client renders and the
// server can process — just like a module.
func (f Field) TableFieldset(columns []Field) Field {
	f.TableColumns = columns
	return f
}

// TableSource names a database table (expected to carry module_id and row_id
// columns) that rows are loaded from when no TableData func is set.
func (f Field) TableSource(table string) Field {
	f.TableSourceName = table
	return f
}

// TableData sets a manual row provider. It receives the current record's values
// as context (e.g. the sibling "module" select) and returns the rows. It takes
// priority over TableSource.
func (f Field) TableData(fn func(ctx map[string]interface{}) []map[string]interface{}) Field {
	f.TableDataFunc = fn
	return f
}

// TableOnSubmit sets a hook that processes the submitted rows before they're
// stored into the module. Its return value becomes the stored column value.
func (f Field) TableOnSubmit(fn func(rows []map[string]interface{}) interface{}) Field {
	f.TableSubmitFunc = fn
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

func (f *Field) GetSQL() string {
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
