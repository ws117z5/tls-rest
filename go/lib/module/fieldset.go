package module

const MODE_LIST = 0b000001
const MODE_VIEW = 0b000010
const MODE_EDIT = 0b000100
const MODE_LOG = 0b001000
const MODE_MULTIPLEUPDATE = 0b010000
const MODE_SUBMIT = 0b100000
const MODE_READONLY = 0b000011
const MODE_EDITSUBMIT = 0b100100 // MODE_EDIT + MODE_SUBMIT
const MODE_ALL = 0b111111

const TYPE_HTML = "Html"

const TYPE_CHECKBOX = "Checkbox"
const TYPE_CHECKBOX_SET = "CheckboxSet"
const TYPE_CHECKBOX_AJAX = "CheckboxAjax"

const TYPE_STRING = "String"
const TYPE_AUTOCOMPLETE = "Autocomplete"

const TYPE_TEXT = "Text"
const TYPE_JSON = "Json"
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
}

func NewField(name, fieldType string, required bool) Field {
	ret := Field{
		Name:       name,
		Type:       fieldType,
		Required:   required,
		Label:      name, // Default label is the field name
		Filterable: true, // Default to filterable
		Sortable:   true, // Default to sortable
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

func (f Field) WithValidation(key string, value interface{}) Field {
	f.Validation[key] = value
	return f
}

func (f Field) WithOption(key string, value interface{}) Field {
	f.Options[key] = value
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
