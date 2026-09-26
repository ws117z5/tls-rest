package module

import (
	"fmt"
	"net/http"
	"strings"

	"tls-rest/go/engine/controllers/functions"
	"tls-rest/go/engine/controllers/request"
)

// moduleControllers maps module id -> its controller, so cross-module checks (comments, likes, images) can ask "may this viewer see/edit that row".
var moduleControllers = map[string]*BaseController{}

// scopedRow returns cols of the record keyCol=keyVal only if it lies in the viewer's read scope (sharing lists, owner scoping, access level, soft delete); nil otherwise.
func (bc *BaseController) scopedRow(r *http.Request, v viewer, cols, keyCol string, keyVal interface{}) (map[string]interface{}, error) {
	// A bare probe request keeps the caller's own query/body from steering the scope builder.
	stub, err := http.NewRequestWithContext(r.Context(), http.MethodGet, "/", nil)
	if err != nil {
		return nil, err
	}
	engine := bc.Engine.WithRequest(request.WithRequest(stub, request.New(stub)))

	var args []interface{}
	argIndex := 1
	conds := engine.buildScopeConditions(&QueryParams{}, v, &argIndex, &args)
	conds = append(conds, fmt.Sprintf("%s = $%d", keyCol, argIndex))
	args = append(args, keyVal)

	db, err := bc.Engine.Module.getDB(r.Context())
	if err != nil {
		return nil, err
	}
	return db.GetOne("SELECT "+cols+" FROM "+engine.TableName+" WHERE "+strings.Join(conds, " AND "), args...)
}

// authorizeRowWrite returns 0 when the caller may modify record id (as addressed by the module's key), else the status to answer with.
func (bc *BaseController) authorizeRowWrite(r *http.Request, id string) int {
	key, val := bc.recordKey(id)
	return bc.authorizeWriteBy(r, key, val)
}

// authorizeWriteBy: 404 outside the caller's read scope, 403 when a sharing/owner-scoped module's row belongs to someone else, admins always pass.
func (bc *BaseController) authorizeWriteBy(r *http.Request, keyCol string, keyVal interface{}) int {
	v := viewerForModule(r, bc.Module.ID)
	if v.isAdmin {
		return 0
	}

	col := "1 AS ok"
	authorOnly := bc.Module.OwnerScoped || bc.Module.VisibilityUsersField != "" || bc.Module.VisibilityGroupsField != ""
	if authorOnly && bc.Engine.hasField("created_by") {
		col = "created_by"
	}
	row, err := bc.scopedRow(r, v, col, keyCol, keyVal)
	if err != nil {
		return http.StatusInternalServerError
	}
	if row == nil {
		return http.StatusNotFound
	}
	if col == "created_by" && (v.userID <= 0 || functions.Int(row["created_by"]) != v.userID) {
		return http.StatusForbidden
	}
	return 0
}

// CanViewRow reports whether the request's viewer may see record id of module moduleID under that module's full row scope
// (sharing lists, owner scoping, access level). Unknown modules are denied.
func CanViewRow(r *http.Request, moduleID string, id int64) bool {
	bc := moduleControllers[moduleID]
	if bc == nil {
		return false
	}
	row, err := bc.scopedRow(r, viewerForModule(r, moduleID), "1 AS ok", "id", id)
	return err == nil && row != nil
}

// CanEditRow is CanViewRow's write counterpart: the same checks Edit applies (author-only on owner-scoped/sharing modules).
func CanEditRow(r *http.Request, moduleID string, id int64) bool {
	bc := moduleControllers[moduleID]
	return bc != nil && bc.authorizeWriteBy(r, "id", id) == 0
}
