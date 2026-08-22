package module

import "tls-rest/go/engine/controllers/field"

// buildDeclaredFilterConditions applies the module's declared list filters
// (<module>/filters.go) to the current list query. The condition-building itself
// lives in the field package (field.BuildFilterConditions); this method just
// supplies the module's filters and the request-scoped visibility predicate — a
// viewer must not be able to filter by a column they can't see.
func (fe *FieldsetEngine) buildDeclaredFilterConditions(argIndex *int, args *[]interface{}) []string {
	if fe.Module == nil || fe.Module.Filters == nil || fe.Request == nil {
		return nil
	}
	v := viewerFromRequest(fe.Request)
	return field.BuildFilterConditions(
		fe.Module.Filters.Fields,
		fe.Request.URL.Query(),
		v.fieldVisibleInSchema,
		argIndex, args,
	)
}
