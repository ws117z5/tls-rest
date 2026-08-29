package field

// Shared, reusable field factories — the equivalent of the legacy framework's
// CommonFields / GeneralFields. Compose these instead of re-declaring the same
// field shape in every module. Each returns a Field you can further refine with
// the builder methods, e.g.:
//
//	CF_Amount("TruckingBuyingRate", "Buying Rate").WithUnit("USD")

// CF_ID builds a read-only integer id column, list/view only.
func ID() Field {
	return NewField("id", TYPE_INT, false).
		WithLabel("ID").
		AsReadOnly().
		InModes(MODE_LIST | MODE_VIEW)
}

// CF_Name builds a required short-text name field.
func Name(label string) Field {
	if label == "" {
		label = "Name"
	}
	return NewField("Name", TYPE_STRING, true).WithLabel(label)
}

// CF_String builds a plain single-line string field.
func String(name, label string) Field {
	return NewField(name, TYPE_STRING, false).WithLabel(label)
}

// CF_Text builds a multi-line text field.
func Text(name, label string) Field {
	return NewField(name, TYPE_TEXT, false).WithLabel(label)
}

// CF_Email builds an email field with a placeholder.
func Email(name, label string) Field {
	if label == "" {
		label = "Email"
	}
	return NewField(name, TYPE_STRING, false).WithLabel(label).WithPlaceholder("name@example.com")
}

// CF_Date builds a date field.
func Date(name, label string) Field {
	return NewField(name, TYPE_DATE, false).WithLabel(label)
}

// CF_DateTime builds a date-time field.
func DateTime(name, label string) Field {
	return NewField(name, TYPE_DATE_TIME, false).WithLabel(label)
}

// CF_Checkbox builds a boolean checkbox field.
func Checkbox(name, label string) Field {
	return NewField(name, TYPE_CHECKBOX, false).WithLabel(label)
}

// CF_Int builds an integer field, right-aligned in the list.
func Int(name, label string) Field {
	return NewField(name, TYPE_INT, false).WithLabel(label).WithAlign("right")
}

// CF_Amount builds a decimal amount field: right-aligned, zero shown blank, and
// negatives styled with the "bad" class (matching the legacy convention).
func Amount(name, label string) Field {
	return NewField(name, TYPE_FLOAT, false).
		WithLabel(label).
		WithAlign("right").
		WithZeroEmpty().
		WithSignClasses("bad", "")
}

// CF_Money builds an amount field with a currency unit suffix.
func CF_Money(name, label, currency string) Field {
	return Amount(name, label).WithUnit(currency)
}

// CF_Select builds a select field with static options.
func Select(name, label string, options []map[string]interface{}) Field {
	return NewField(name, TYPE_SELECT, false).
		WithLabel(label).
		WithOption("options", options)
}

// CF_SelectDynamic builds a select whose options are resolved at request time.
func SelectDynamic(name, label string, provider func() []map[string]interface{}) Field {
	return NewField(name, TYPE_SELECT, false).WithLabel(label).WithOptions(provider)
}

// CF_Link builds an integer foreign-key field rendered as a link into the target
// module's record.
func Link(name, label, module string) Field {
	return NewField(name, TYPE_INT, false).WithLabel(label).WithLink(module)
}

// CF_CreatedBy builds the read-only "created_by" system field (list/view only).
func CreatedBy() Field {
	return NewField("created_by", TYPE_INT, false).
		WithLabel("Created By").
		AsReadOnly().
		InModes(MODE_LIST | MODE_VIEW)
}

// CF_Created builds the read-only "created" timestamp field (list/view only).
func Created() Field {
	return NewField("created", TYPE_DATE_TIME, false).
		WithLabel("Created").
		AsReadOnly().
		InModes(MODE_LIST | MODE_VIEW)
}
