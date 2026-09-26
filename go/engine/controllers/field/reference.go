package field

import "fmt"

// Reference ties a destination field to the stored id column that points at its value in another table.
type Reference struct {
	StoredName string
	StoredType string
	Table      string
}

// AsReference makes f the destination of a reference: a virtual, read-only value pulled from another table through the
// stored id column storedName. The module adds that column when it initializes, unless it declares one itself.
// Builders chained afterwards override the defaults (list and view modes, not sortable, not searchable):
//
//	NewField("compiled_html", TYPE_HTML, false).AsReference("html_id", TYPE_INT).DestinationTable("html").InModes(MODE_VIEW)
func (f Field) AsReference(storedName, storedType string) Field {
	f = f.AsVirtual().AsReadOnly().NonSortable().NonSearchable().InModes(MODE_LIST | MODE_VIEW)
	ref := Reference{}
	if f.Ref != nil {
		ref = *f.Ref
	}
	ref.StoredName, ref.StoredType = storedName, storedType
	f.Ref = &ref
	return f
}

// DestinationTable names the table holding the referenced value (key column "id", value column named like this field).
// Unless WithSQL is set, the value is (SELECT <field> FROM <table> WHERE <table>.id = <stored column>).
func (f Field) DestinationTable(table string) Field {
	ref := Reference{}
	if f.Ref != nil {
		ref = *f.Ref
	}
	ref.Table = table
	f.Ref = &ref
	return f
}

// ExpandReferences finishes every reference field (default lookup SQL) and inserts the stored id column each one
// needs just before it, unless the list already declares that column. Called by module initialization.
func ExpandReferences(fields []Field) []Field {
	declared := make(map[string]bool, len(fields))
	for _, f := range fields {
		declared[f.Name] = true
	}

	out := make([]Field, 0, len(fields)+2)
	for _, f := range fields {
		ref := f.Ref
		if ref == nil {
			out = append(out, f)
			continue
		}
		if ref.StoredName == "" {
			panic(fmt.Sprintf("reference field %q: no stored column (AsReference)", f.Name))
		}
		if f.SQL == "" {
			if ref.Table == "" {
				panic(fmt.Sprintf("reference field %q: set DestinationTable or WithSQL", f.Name))
			}
			f.SQL = fmt.Sprintf("(SELECT %s FROM %s WHERE %s.id = %s)", f.Name, ref.Table, ref.Table, ref.StoredName)
		}
		if !declared[ref.StoredName] {
			out = append(out, NewField(ref.StoredName, ref.StoredType, false).AsReadOnly().InModes(MODE_VIEW))
			declared[ref.StoredName] = true
		}
		out = append(out, f)
	}
	return out
}
