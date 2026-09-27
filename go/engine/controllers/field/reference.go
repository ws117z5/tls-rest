package field

import (
	"fmt"
	"strings"
)

// Reference ties a destination field to the stored id column that points at its value in another table.
type Reference struct {
	StoredName string
	StoredType string
	DestColumn string // the column of Table holding the value; the field keeps its own name
	Table      string
	// Alias is set by ExpandReferences when several fields read the same remote row; they then share one lateral join under this alias.
	Alias string
}

// AsReference makes f the destination of a reference: a virtual, read-only value pulled from another table through the
// stored id column storedName (of storedType), reading the destColumn column of the remote row. The field keeps its own
// name, so several fields can read the same remote column through different stored columns. The module adds the stored
// column when it initializes, unless it declares one itself. Builders chained afterwards override the defaults (list and
// view modes, not sortable, not searchable):
//
//	NewField("compiled_html", TYPE_HTML, false).AsReference("html_id", TYPE_INT, "compiled_html").DestinationTable("html").InModes(MODE_VIEW)
func (f Field) AsReference(storedName, storedType, destColumn string) Field {
	f = f.AsVirtual().AsReadOnly().NonSortable().NonSearchable().InModes(MODE_LIST | MODE_VIEW)
	ref := Reference{}
	if f.Ref != nil {
		ref = *f.Ref
	}
	ref.StoredName, ref.StoredType, ref.DestColumn = storedName, storedType, destColumn
	f.Ref = &ref
	return f
}

// DestinationTable names the table holding the referenced value (key column "id").
// Unless WithSQL is set, the value is (SELECT <dest column> FROM <table> WHERE <table>.id = <stored column>).
func (f Field) DestinationTable(table string) Field {
	ref := Reference{}
	if f.Ref != nil {
		ref = *f.Ref
	}
	ref.Table = table
	f.Ref = &ref
	return f
}

// ExpandReferences finishes every reference field and inserts the stored id column each one needs just before it,
// unless the list already declares that column. A lone reference gets a correlated subselect; two or more default-SQL
// references sharing a stored column and table share one lateral join instead (see ReferenceJoins), so the remote row
// is fetched once. Called by module initialization.
func ExpandReferences(fields []Field) []Field {
	declared := make(map[string]bool, len(fields))
	shared := map[string]int{}
	for _, f := range fields {
		declared[f.Name] = true
		if f.Ref != nil && f.SQL == "" && f.Ref.Table != "" {
			shared[groupKey(f.Ref)]++
		}
	}

	out := make([]Field, 0, len(fields)+2)
	for _, f := range fields {
		ref := f.Ref
		if ref == nil {
			out = append(out, f)
			continue
		}
		if ref.StoredName == "" || ref.DestColumn == "" {
			panic(fmt.Sprintf("reference field %q: AsReference needs a stored column and a destination column", f.Name))
		}
		if f.SQL == "" {
			if ref.Table == "" {
				panic(fmt.Sprintf("reference field %q: set DestinationTable or WithSQL", f.Name))
			}
			if shared[groupKey(ref)] > 1 {
				joined := *ref
				joined.Alias = "ref_" + ref.StoredName
				f.Ref = &joined
				f.SQL = joined.Alias + "." + f.Name
			} else {
				f.SQL = fmt.Sprintf("(SELECT %s FROM %s WHERE %s.id = %s)", ref.DestColumn, ref.Table, ref.Table, ref.StoredName)
			}
		}
		if !declared[ref.StoredName] {
			out = append(out, NewField(ref.StoredName, ref.StoredType, false).AsReadOnly().InModes(MODE_VIEW))
			declared[ref.StoredName] = true
		}
		out = append(out, f)
	}
	return out
}

func groupKey(r *Reference) string { return r.StoredName + "|" + r.Table }

// ReferenceJoins returns the JOIN clause (leading space included, "" when none) that the selected fields need:
// one LEFT JOIN LATERAL per alias set by ExpandReferences, exposing only the selected fields' columns under their
// field names, so unqualified base-table columns elsewhere in the query stay unambiguous. baseTable owns the stored columns.
func ReferenceJoins(baseTable string, selected []Field) string {
	type group struct {
		table, stored string
		cols          []string
	}
	groups := map[string]*group{}
	var order []string
	for _, f := range selected {
		if f.Ref == nil || f.Ref.Alias == "" {
			continue
		}
		g := groups[f.Ref.Alias]
		if g == nil {
			g = &group{table: f.Ref.Table, stored: f.Ref.StoredName}
			groups[f.Ref.Alias] = g
			order = append(order, f.Ref.Alias)
		}
		g.cols = append(g.cols, fmt.Sprintf("%s.%s AS %s", g.table, f.Ref.DestColumn, f.Name))
	}

	var sb strings.Builder
	for _, alias := range order {
		g := groups[alias]
		fmt.Fprintf(&sb, " LEFT JOIN LATERAL (SELECT %s FROM %s WHERE %s.id = %s.%s) AS %s ON true",
			strings.Join(g.cols, ", "), g.table, g.table, baseTable, g.stored, alias)
	}
	return sb.String()
}
