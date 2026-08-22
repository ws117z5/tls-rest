package field

/*
type Params map[string]interface{}

func Id(params Params) Field {
	defaultParams := Params{
		"modes":          MODE_LIST,
		"tableAlign":     "left",
		"tableWidthHtml": 24,
	}
	for k, v := range params {
		defaultParams[k] = v
	}
	return NewFieldAbstract("ID", TYPE_INT, defaultParams)
}

func Color(params Params) Field {
	defaultParams := Params{
		"modes":              MODE_ALL &^ MODE_LIST,
		"sqlSelectMandatory": true,
	}
	for k, v := range params {
		defaultParams[k] = v
	}
	return NewFieldAbstract("Color", TYPE_COLOR, defaultParams)
}

func Active(params Params) Field {
	return NewFieldAbstract("Active", TYPE_ACTIVE_INACTIVE, params)
}

func ValidFrom(params Params) Field {
	defaultParams := Params{"name": "Valid From"}
	for k, v := range params {
		defaultParams[k] = v
	}
	return NewFieldAbstract("ValidFrom", TYPE_DATE, defaultParams)
}

func ValidUntil(params Params) Field {
	defaultParams := Params{
		"name": "Valid Until",
		"getDataFromRequestFunction": func(request map[string]interface{}, value interface{}) interface{} {
			if value == nil || value == "" {
				return "2099-01-01"
			}
			return value
		},
	}
	for k, v := range params {
		defaultParams[k] = v
	}
	return NewFieldAbstract("ValidUntil", TYPE_DATE, defaultParams)
}

func DateField(prefix string, params Params) Field {
	fieldKey := prefix + "Date"
	defaultParams := Params{}
	if prefix != "" {
		defaultParams["name"] = prefix + " Date"
	}
	for k, v := range params {
		defaultParams[k] = v
	}
	return NewFieldAbstract(fieldKey, TYPE_DATE, defaultParams)
}

func Amount(prefix string, params Params) Field {
	fieldKey := prefix + "Amount"
	defaultParams := Params{"tableWidthExcel": 12}
	if prefix != "" {
		defaultParams["name"] = prefix + " Amount"
	}
	for k, v := range params {
		defaultParams[k] = v
	}
	return NewFieldAbstract(fieldKey, TYPE_MONEY, defaultParams)
}

func AmountWithCurrency(prefix string, params Params) Field {
	fieldKey := prefix + "Amount"
	defaultParams := Params{}
	if prefix != "" {
		defaultParams["name"] = prefix + " Amount"
	}
	for k, v := range params {
		defaultParams[k] = v
	}
	return NewFieldAbstract(fieldKey, TYPE_MONEY_WITH_CURRENCY, defaultParams)
}

func Currency(prefix string, params Params) Field {
	fieldKey := prefix + "CurrencyISO"
	defaultParams := Params{
		"tableAlign":        "center",
		"tableWidthHtml":    60,
		"empty":             false,
		"options":           "SELECT Name, Name FROM currency ORDER BY OrderID, Name",
		"editCssClass":      "select-currency",
		"name":              "Currency",
		"nameTable":         "Curr.",
		"title":             "Currency",
		"nameTableSettings": "Currency",
	}
	if prefix != "" {
		defaultParams["name"] = prefix + " Currency"
		defaultParams["title"] = prefix + " Currency"
		defaultParams["nameTable"] = prefix + " Curr."
		defaultParams["nameTableSettings"] = prefix + " Currency"
	}
	for k, v := range params {
		defaultParams[k] = v
	}
	return NewFieldAbstract(fieldKey, TYPE_SELECT, defaultParams)
}

func Comments(params Params) Field {
	return NewFieldAbstract("Comments", TYPE_TEXT, params)
}

func OrderId() Field {
	return NewFieldAbstract("OrderID", TYPE_INT, Params{
		"name":           "Order ID",
		"editAlign":      "left",
		"tableAlign":     "center",
		"tableWidthHtml": 50,
	})
}

func GrossWeight() Field {
	return NewFieldAbstract("GrossWeight", TYPE_FLOAT, Params{
		"name":           "Gross Weight",
		"nameTable":      "Gross\nWeight (kg)",
		"tableWidthHtml": 62,
		"editCssClass":   "input-weight",
		"uom":            "kg",
	})
}

func NetWeight() Field {
	return NewFieldAbstract("NetWeight", TYPE_FLOAT, Params{
		"name":           "Net Weight",
		"nameTable":      "Net\nWeight (kg)",
		"tableWidthHtml": 62,
		"editCssClass":   "input-weight",
		"uom":            "kg",
	})
}

func ChargeableWeight() Field {
	return NewFieldAbstract("ChargeableWeight", TYPE_FLOAT, Params{
		"name":           "Chargeable Weight",
		"tableWidthHtml": 62,
		"editCssClass":   "input-weight",
		"uom":            "kg",
	})
}

func Width() Field {
	return NewFieldAbstract("Width", TYPE_INT, Params{"uom": "cm"})
}

func Height() Field {
	return NewFieldAbstract("Height", TYPE_INT, Params{"uom": "cm"})
}

func Length() Field {
	return NewFieldAbstract("Length", TYPE_INT, Params{"uom": "cm"})
}

func Quantity() Field {
	return NewFieldAbstract("Quantity", TYPE_INT, Params{})
}

func Volume() Field {
	return NewFieldAbstract("Volume", TYPE_FLOAT, Params{
		"name":           "Volume",
		"tableWidthHtml": 62,
		"editCssClass":   "input-weight",
		"uom":            "m<sup>3</sup>",
		"precision":      3,
	})
}

func Pieces() Field {
	return NewFieldAbstract("Pieces", TYPE_INT, Params{"name": "Pieces"})
}

func Address() Field {
	return NewFieldAbstract("Address", TYPE_TEXT, Params{"clickable": true})
}

func Email() Field {
	return NewFieldAbstract("Email", TYPE_STRING, Params{
		"name":           "E-mail",
		"editCssClass":   "email",
		"clickable":      true,
		"tableWidthHtml": 126,
	})
}

func Phone(fieldKey string, params Params) Field {
	defaultParams := Params{
		"name":           "Phone",
		"editCssClass":   "phone",
		"tableWidthHtml": 126,
	}
	for k, v := range params {
		defaultParams[k] = v
	}
	return NewFieldAbstract(fieldKey, TYPE_STRING, defaultParams)
}

*/
