package field

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// This file adds declarative *list filters* on top of the existing fieldset.
//
// A module declares the filters it accepts in list mode as an ordinary fieldset,
// conventionally in <module>/filters.go:
//
//	func (p *Posts) filters() *module.Filedset {
//	    return module.NewFieldset(
//	        module.NewFilter("title", module.TYPE_STRING).Contains(),
//	        module.NewFilter("public", module.TYPE_CHECKBOX).Equals(),
//	    )
//	}
//
// and assigns it to Module.Filters before Initialize(). On GET /<module> the
// fieldset engine reads each declared filter's value from the request query and
// turns it into a WHERE clause for the list query.
//
// Filters are a strict allow-list: only declared filters are honoured, so an
// arbitrary query parameter can never reach the SQL. A filter is a normal Field
// (Name = the query parameter, Type = value casting) that is virtual — it is
// only ever a WHERE clause, never part of a SELECT projection.

// ListFilters is the set of filters a module accepts in list mode. It is an
// alias for Filedset so filters.go reads naturally while reusing the fieldset
// type the rest of the engine already understands.
type ListFilters = Filedset

// Filter text-match modes. Stored under Options["filterMatch"] and used to wrap
// the bound value for ILIKE comparisons.
const (
	filterMatchExact    = ""
	filterMatchContains = "contains"
	filterMatchPrefix   = "prefix"
	filterMatchSuffix   = "suffix"
)

// NewFilter builds a list filter.
//
// name is the query parameter the client sends (?name=value); fieldType drives
// value casting and validation. By default a filter compares for equality
// against a column named after the filter. Use WithSQL to target a different
// column, the operator helpers (Contains, GreaterOrEqual, ...) to change the
// comparison, or WithSQLWhere for a fully custom clause.
func NewFilter(name, fieldType string) Field {
	f := NewField(name, fieldType, false)
	f.Filterable = true
	f.Virtual = true   // never selected as a column, only used in WHERE
	f.Mode = MODE_LIST // filters only apply to list mode
	return f
}

// filterColumn returns the SQL column a filter compares against (SQL if set,
// otherwise the filter name).
func filterColumn(f Field) string {
	if f.SQL != "" {
		return f.SQL
	}
	return f.Name
}

// Operator helpers set SQLWhere to a clause containing a single %s verb, which
// the engine replaces with the positional placeholder ($1, $2, ...). This is the
// same convention buildFilterConditions already uses for field-level SQLWhere.

// Equals matches rows where the column equals the value (the default behaviour).
func (f Field) Equals() Field {
	f.SQLWhere = filterColumn(f) + " = %s"
	return f
}

// NotEquals matches rows where the column differs from the value.
func (f Field) NotEquals() Field {
	f.SQLWhere = filterColumn(f) + " <> %s"
	return f
}

// Contains does a case-insensitive substring match (col ILIKE '%value%').
func (f Field) Contains() Field {
	f.SQLWhere = filterColumn(f) + " ILIKE %s"
	return f.withFilterMatch(filterMatchContains)
}

// StartsWith does a case-insensitive prefix match (col ILIKE 'value%').
func (f Field) StartsWith() Field {
	f.SQLWhere = filterColumn(f) + " ILIKE %s"
	return f.withFilterMatch(filterMatchPrefix)
}

// EndsWith does a case-insensitive suffix match (col ILIKE '%value').
func (f Field) EndsWith() Field {
	f.SQLWhere = filterColumn(f) + " ILIKE %s"
	return f.withFilterMatch(filterMatchSuffix)
}

// GreaterOrEqual matches rows where column >= value (e.g. a "from" date bound).
func (f Field) GreaterOrEqual() Field {
	f.SQLWhere = filterColumn(f) + " >= %s"
	return f
}

// LessOrEqual matches rows where column <= value (e.g. a "to" date bound).
func (f Field) LessOrEqual() Field {
	f.SQLWhere = filterColumn(f) + " <= %s"
	return f
}

func (f Field) withFilterMatch(mode string) Field {
	if f.Options == nil {
		f.Options = make(map[string]interface{})
	}
	f.Options["filterMatch"] = mode
	return f
}

func (f Field) FilterMatch() string {
	if f.Options != nil {
		if m, ok := f.Options["filterMatch"].(string); ok {
			return m
		}
	}
	return filterMatchExact
}

// BuildFilterConditions turns declared list filters into SQL WHERE conditions,
// reading each value from q. `visible` reports whether the caller may filter by
// a given field (a column the viewer can't see must not be filterable); pass nil
// to allow all. The visibility predicate is injected so this package stays free
// of request/session/module types. Nothing is appended for filters whose value
// is absent, empty, invalid for its type, or hidden.
func BuildFilterConditions(filters []Field, q url.Values, visible func(Field) bool, argIndex *int, args *[]interface{}) []string {
	var conditions []string

	for _, f := range filters {
		raw := filterParamValue(q, f.Name)
		if raw == "" {
			continue
		}

		if visible != nil && !visible(f) {
			continue
		}

		value, ok := castFilterValue(f, raw)
		if !ok {
			// Ignore values that don't match the declared type rather than
			// failing the whole list request.
			continue
		}

		placeholder := fmt.Sprintf("$%d", *argIndex)
		if f.SQLWhere != "" {
			conditions = append(conditions, fmt.Sprintf(f.SQLWhere, placeholder))
		} else {
			conditions = append(conditions, fmt.Sprintf("%s = %s", filterColumn(f), placeholder))
		}
		*args = append(*args, value)
		*argIndex++
	}

	return conditions
}

// filterParamValue reads a filter value from the query, accepting both the plain
// (?name=value) and the legacy bracket (?filters[name]=value) forms.
func filterParamValue(q url.Values, name string) string {
	if val := strings.TrimSpace(q.Get(name)); val != "" {
		return val
	}
	return strings.TrimSpace(q.Get("filters[" + name + "]"))
}

// castFilterValue converts a raw string parameter into a typed value suitable
// for binding and applies LIKE wrapping for text-match filters. It returns false
// when the value is not valid for the declared type.
func castFilterValue(f Field, raw string) (interface{}, bool) {
	switch f.Type {
	case TYPE_INT, TYPE_MONTH, TYPE_WEEK:
		n, err := strconv.Atoi(raw)
		if err != nil {
			return nil, false
		}
		return n, true
	case TYPE_FLOAT, TYPE_MONEY, TYPE_MONEY_WITH_CURRENCY:
		x, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, false
		}
		return x, true
	case TYPE_CHECKBOX, TYPE_YES_NO, TYPE_ACTIVE_INACTIVE:
		return parseFilterBool(raw), true
	default:
		return wrapLikeValue(f, raw), true
	}
}

// wrapLikeValue wraps a string value with % markers according to the filter's
// text-match mode, so the ILIKE placeholder receives e.g. "%term%".
func wrapLikeValue(f Field, raw string) string {
	switch f.FilterMatch() {
	case filterMatchContains:
		return "%" + raw + "%"
	case filterMatchPrefix:
		return raw + "%"
	case filterMatchSuffix:
		return "%" + raw
	default:
		return raw
	}
}

func parseFilterBool(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on", "active", "y", "t":
		return true
	default:
		return false
	}
}
