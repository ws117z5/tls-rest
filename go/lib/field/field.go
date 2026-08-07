package field

import (
	"fmt"
	"html"
	"strings"
)

type Mode int

const (
	MODE_LIST Mode = 1 << iota
	MODE_VIEW
	MODE_EDIT
	MODE_LOG
	MODE_MULTIPLEUPDATE
	MODE_SUBMIT
	MODE_READONLY   = MODE_LIST | MODE_VIEW
	MODE_EDITSUBMIT = MODE_EDIT | MODE_SUBMIT
	MODE_ALL        = 0b111111
)

type FieldType string

const (
	TYPE_HTML                FieldType = "Html"
	TYPE_CHECKBOX            FieldType = "Checkbox"
	TYPE_CHECKBOX_SET        FieldType = "CheckboxSet"
	TYPE_CHECKBOX_AJAX       FieldType = "CheckboxAjax"
	TYPE_STRING              FieldType = "String"
	TYPE_AUTOCOMPLETE        FieldType = "Autocomplete"
	TYPE_TEXT                FieldType = "Text"
	TYPE_JSON                FieldType = "Json"
	TYPE_AUTOCOMPLETE_TEXT   FieldType = "AutocompleteText"
	TYPE_DATE                FieldType = "Date"
	TYPE_DATE_TIME           FieldType = "DateTime"
	TYPE_COLOR               FieldType = "Color"
	TYPE_WEEK                FieldType = "Week"
	TYPE_INT                 FieldType = "Int"
	TYPE_FLOAT               FieldType = "Float"
	TYPE_MONEY               FieldType = "Money"
	TYPE_MONEY_WITH_CURRENCY FieldType = "MoneyWithCurrency"
	TYPE_SELECT              FieldType = "Select"
	TYPE_SELECT_ADDNEW       FieldType = "SelectAddNew"
	TYPE_SELECT_CHILD        FieldType = "SelectChild"
	TYPE_SELECT2_MULTIPLE    FieldType = "Select2Multiple"
	TYPE_ACTIVE_INACTIVE     FieldType = "ActiveInactive"
	TYPE_YES_NO              FieldType = "YesNo"
	TYPE_MONTH               FieldType = "Month"
)

type Params map[string]interface{}

type Field interface {
	GetParam(name string) interface{}
	SetParam(name string, value interface{})
	GetType() FieldType
	GetHtml(value string, data map[string]interface{}, forTable bool) string
	HasMode(mode Mode) bool
	Clone(newKey string, newParams Params) Field
}

type FieldAbstract struct {
	Key               string
	DefaultParams     Params
	UserDefinedParams Params
	Type              FieldType
}

func NewFieldAbstract(key string, typ FieldType, userParams Params) *FieldAbstract {
	def := Params{
		"modes":               MODE_ALL,
		"tableAlign":          "left",
		"editMandatory":       false,
		"nobr":                false,
		"clickable":           false,
		"submitToupper":       false,
		"groupconcat":         false,
		"groupconcatTruncate": false,
	}
	for k, v := range userParams {
		def[k] = v
	}
	return &FieldAbstract{
		Key:               key,
		DefaultParams:     def,
		UserDefinedParams: userParams,
		Type:              typ,
	}
}

func (f *FieldAbstract) SetParam(name string, value interface{}) {
	f.UserDefinedParams[name] = value
}

func (f *FieldAbstract) GetParam(name string) interface{} {
	if val, ok := f.UserDefinedParams[name]; ok {
		return val
	}
	if val, ok := f.DefaultParams[name]; ok {
		return val
	}
	switch name {
	case "name":
		return f.Key
	case "nameDbLog":
		return f.GetParam("name")
	case "nameTable":
		val := fmt.Sprintf("%v", f.GetParam("name"))
		uom := fmt.Sprintf("%v", f.GetParam("uom"))
		if uom != "" && uom != "<nil>" {
			val += "\n(" + uom + ")"
		}
		return val
	case "nameTableSettings":
		val := fmt.Sprintf("%v", f.GetParam("nameTable"))
		val = strings.ReplaceAll(val, "<br>", " ")
		val = strings.ReplaceAll(val, "\n", " ")
		return val
	case "tableWidthExcel":
		if strings.HasPrefix(f.Key, "Name") {
			return 30
		}
	case "tableOrderBy":
		return f.Key
	case "editName":
		return f.Key
	case "editId":
		return f.Key
	}
	return nil
}

func (f *FieldAbstract) GetType() FieldType {
	return f.Type
}

func (f *FieldAbstract) HasMode(mode Mode) bool {
	modes, ok := f.GetParam("modes").(Mode)
	if !ok {
		if i, ok := f.GetParam("modes").(int); ok {
			modes = Mode(i)
		}
	}
	return modes&mode != 0
}

func (f *FieldAbstract) Clone(newKey string, newParams Params) Field {
	merged := Params{}
	for k, v := range f.UserDefinedParams {
		merged[k] = v
	}
	for k, v := range newParams {
		merged[k] = v
	}
	if newKey == "" {
		newKey = f.Key
	}
	return NewFieldAbstract(newKey, f.Type, merged)
}

// Example: minimal HTML output
func (f *FieldAbstract) GetHtml(value string, data map[string]interface{}, forTable bool) string {
	return html.EscapeString(value)
}

// You can extend FieldAbstract for concrete types and override GetHtml, etc.
