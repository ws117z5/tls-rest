package field

import (
	"context"
	"fmt"
)

const MODE_LIST = 0b00000001
const MODE_VIEW = 0b00000010
const MODE_CREATE = 0b00000100
const MODE_EDIT = 0b00001000
const MODE_DELETE = 0b00010000
const MODE_LOG = 0b00100000
const MODE_MULTIPLEUPDATE = 0b01000000
const MODE_SUBMIT = 0b10000000
const MODE_READONLY = MODE_LIST | MODE_VIEW
const MODE_EDITSUBMIT = MODE_EDIT | MODE_SUBMIT
const MODE_ALL = 0b11111111

const TYPE_HTML = "Html"

const TYPE_CHECKBOX = "Checkbox"
const TYPE_CHECKBOX_SET = "CheckboxSet"
const TYPE_CHECKBOX_AJAX = "CheckboxAjax"

const TYPE_STRING = "String"
const TYPE_UUID = "Uuid"

const TYPE_TEXT = "Text"
const TYPE_MARKDOWN = "Markdown"
const TYPE_JSON = "Json"
const TYPE_IMAGE = "Image"

const TYPE_DATE = "Date"
const TYPE_DATE_TIME = "DateTime"
const TYPE_COLOR = "Color"
const TYPE_WEEK = "Week"
const TYPE_TIME_DURATION = "TimeDuration"
const TYPE_PASSWORD = "Password"

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
	Name            string                 `json:"name"`
	Type            string                 `json:"type"`
	Required        bool                   `json:"required"`
	SQL             string                 `json:"sql"`                  // SQL column definition or expression
	SQLWhere        string                 `json:"sql_where"`            // Custom SQL WHERE clause template
	SQLWhereNegated string                 `json:"sql_where_not"`        // SQLWhere's negated form, for a -"phrase" filter value
	Filterable      bool                   `json:"filterable"`           // Whether this field can be used for filtering
	Sortable        bool                   `json:"sortable"`             // Whether this field can be used for sorting
	Searchable      bool                   `json:"searchable"`           // Whether this field is included in search
	Virtual         bool                   `json:"virtual"`              // Whether this field is virtual (not stored in DB)
	ReadOnly        bool                   `json:"readonly"`             // Whether this field is read-only
	AdminOnly       bool                   `json:"admin_only,omitempty"` // Visible/appliable only to admins (fields & filters)
	DefaultValue    interface{}            `json:"default_value"`        // Default value for the field
	Validation      map[string]interface{} `json:"validation"`           // Validation rules
	Options         map[string]interface{} `json:"options"`              // Field-specific options (autocomplete URL, etc.)
	Mode            int                    `json:"mode"`                 // Bit flags for which modes this field appears in
	Label           string                 `json:"label"`                // Display label
	Description     string                 `json:"description"`          // Field description
	Placeholder     string                 `json:"placeholder"`          // Input placeholder (edit/create/filters)
	Access          int                    `json:"access"`               // Minimum access level required to see this field (0 = everyone)

	// Display / formatting modifiers (list & view rendering).
	LinkModule    string `json:"linkModule,omitempty"`    // render the value as a link into this module's record
	Unit          string `json:"unit,omitempty"`          // unit suffix shown after the value (e.g. "kg", "USD")
	ZeroEmpty     bool   `json:"zeroEmpty,omitempty"`     // render 0 as blank
	NegativeClass string `json:"negativeClass,omitempty"` // CSS class applied when the value is negative
	PositiveClass string `json:"positiveClass,omitempty"` // CSS class applied when the value is positive
	Align         string `json:"align,omitempty"`         // list column alignment: left|center|right
	ColumnWidth   string `json:"columnWidth,omitempty"`   // list column width (e.g. "120px")

	// TYPE_TABLE configuration (clean API — see TableFieldset/TableSource/
	// TableData/TableOnSubmit/TableRowsAddable builders). The fieldset (columns)
	// is serialized to the client, which renders each column with its own field
	// component; the data/submit hooks are server-side only.
	TableColumns    []Field                                                                         `json:"tableFieldset,omitempty"` // column definitions (each a Field)
	TableSourceName string                                                                          `json:"tableSource,omitempty"`   // DB table with module_id,row_id to load rows from
	TableDataFunc   func(ctx context.Context, data map[string]interface{}) []map[string]interface{} `json:"-"`                       // manual row provider (wins over TableSource)
	TableSubmitFunc func(data []map[string]interface{}) interface{}                                 `json:"-"`                       // process submitted rows before storing
	// RowsAddable lets the client add and remove rows (default: rows are fixed,
	// supplied only by TableDataFunc). RowKey names the column that identifies a
	// row; empty means the first column. Duplicate keys are allowed.
	RowsAddable bool   `json:"tableRowsAddable,omitempty"`
	RowKey      string `json:"tableRowKey,omitempty"`

	// OptionsFunc supplies the field's options at request time (like the legacy
	// framework's dynamic 'options'). Its result is resolved into Options["options"]
	// when the fieldset is served. Set via WithOptions.
	OptionsFunc func() []map[string]interface{} `json:"-"`

	// OptionsCtxFunc is like OptionsFunc but also receives a viewer map
	// describing the requesting user (userID, isAdmin, level), so a field can
	// scope its choices to the caller's authority. Domain policy lives in the
	// module's closure, not the engine. Set via WithOptionsCtx.
	OptionsCtxFunc func(ctx context.Context, viewer map[string]interface{}) []map[string]interface{} `json:"-"`

	// Autocomplete config, set via WithAutocomplete.
	AutocompleteKind   string                                                                              `json:"autocomplete,omitempty"` // "function" | "sql" | "source"
	AutocompleteFunc   func(ctx context.Context, input string, values map[string]interface{}) []AutoOption `json:"-"`
	AutocompleteSQL    string                                                                              `json:"-"`
	AutocompleteSource []string                                                                            `json:"-"` // {table, field, match}
	// AutocompleteViewFunc: TYPE_INT's List/View id->label resolver; unset reuses AutocompleteKind's own search.
	AutocompleteViewFunc func(ctx context.Context, id string, values map[string]interface{}) (string, bool) `json:"-"`

	// Resize configures server-side resizing for TYPE_IMAGE fields, applied on
	// upload. Set via WithResize.
	Resize *ResizeOptions `json:"-"`
}

// ResizeOptions bounds a TYPE_IMAGE field's stored size. Width/Height (px, 0 =
// unconstrained on that axis) scale proportionally when only one is set, or
// fit-within when both are; MaxBytes (0 = unconstrained) re-encodes at lower
// JPEG quality until met.
type ResizeOptions struct {
	Width    int
	Height   int
	MaxBytes int
}

func NewField(name, fieldType string, required bool) Field {
	ret := Field{
		Name:       name,
		Type:       fieldType,
		Required:   required,
		Label:      name,                    // Default label is the field name
		Filterable: fieldType != TYPE_TABLE, // Tables are not filterable by default
		Sortable:   fieldType != TYPE_TABLE, // Tables are not sortable by default
		Searchable: fieldType == TYPE_STRING || fieldType == TYPE_TEXT,
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

// WithSource makes a virtual, read-only field's value come from another
// module's row (e.g. a Legal page field sourced from a specific Posts row's
// content, editable via the Posts admin UI) instead of this table's own
// column — the same SELECT-time SQL as WithSQL+AsVirtual, generated for you.
func (f Field) WithSource(module, sourceField string, id int64) Field {
	f.SQL = fmt.Sprintf("(SELECT %s FROM %s WHERE id = %d)", sourceField, module, id)
	f.Virtual = true
	f.ReadOnly = true
	return f
}

// WithResize configures a TYPE_IMAGE field to be resized on upload; see
// ResizeOptions.
func (f Field) WithResize(opts ResizeOptions) Field {
	f.Resize = &opts
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

// WithAutocomplete enables type-ahead search via /autocomplete/{field}.
// params: one of "function"/"sql"/"source" (search), plus optional "view"
// (TYPE_INT id->label resolver for List/View).

// AutoOption: Value is stored, Label is shown.
type AutoOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

func (f Field) WithAutocomplete(params map[string]interface{}) Field {
	if fn, ok := params["function"].(func(ctx context.Context, input string, values map[string]interface{}) []AutoOption); ok {
		f.AutocompleteKind = "function"
		f.AutocompleteFunc = fn
	} else if q, ok := params["sql"].(string); ok {
		f.AutocompleteKind = "sql"
		f.AutocompleteSQL = q
	} else if s, ok := params["source"].([]string); ok {
		f.AutocompleteKind = "source"
		f.AutocompleteSource = s
	}
	if vf, ok := params["view"].(func(ctx context.Context, id string, values map[string]interface{}) (string, bool)); ok {
		f.AutocompleteViewFunc = vf
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

// WithOptionsCtx sets an options provider that receives a context map with the
// requesting user's authority ("userID", "isAdmin", "level"), so the module can
// scope the choices (e.g. only groups a user may assign). Takes priority over a
// static list and over WithOptions.
func (f Field) WithOptionsCtx(fn func(ctx context.Context, viewer map[string]interface{}) []map[string]interface{}) Field {
	f.OptionsCtxFunc = fn
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
func (f Field) TableData(fn func(ctx context.Context, data map[string]interface{}) []map[string]interface{}) Field {
	f.TableDataFunc = fn
	return f
}

// TableOnSubmit sets a hook that processes the submitted rows before they're
// stored into the module. Its return value becomes the stored column value.
func (f Field) TableOnSubmit(fn func(rows []map[string]interface{}) interface{}) Field {
	f.TableSubmitFunc = fn
	return f
}

// TableRowsAddable lets the user add and remove rows in the table editor (by
// default the row set is fixed and comes only from TableData). keyColumn names
// the column that identifies a row; pass "" to use the first column. Rows added
// by the user start from each column's default value; duplicate keys are allowed.
func (f Field) TableRowsAddable(keyColumn string) Field {
	f.RowsAddable = true
	f.RowKey = keyColumn
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

// AsAdminOnly marks a field/filter visible and appliable only to admins.
func (f Field) AsAdminOnly() Field {
	f.AdminOnly = true
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

// WithExtraParams merges arbitrary field-specific params into Options (e.g.
// WithExtraParams(map[string]interface{}{"format": "mm:ss"}) for a duration).
func (f Field) WithExtraParams(params map[string]interface{}) Field {
	if f.Options == nil {
		f.Options = map[string]interface{}{}
	}
	for k, v := range params {
		f.Options[k] = v
	}
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

// AsRequired marks the field mandatory — a fluent alternative to NewField's
// positional required bool, for when true doesn't read well inline.
func (f Field) AsRequired() Field {
	f.Required = true
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
