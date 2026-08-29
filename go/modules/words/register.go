package words

import (
	"encoding/json"
	"net/http"
	"strconv"

	"tls-rest/go/engine/controllers/db/cache"
	"tls-rest/go/engine/controllers/db/pgdb"
	"tls-rest/go/engine/controllers/functions"

	"github.com/gorilla/mux"
)

// registerReq is the body of POST /words/{id}/register. Either form works:
//
//	{"result": "success"}   {"result": "fail"}   {"success": true}
type registerReq struct {
	Result  string `json:"result"`
	Success *bool  `json:"success,omitempty"`
}

// registerResult records one practice attempt for a word the caller owns:
// tries += 1, and success += 1 (a win) or fail += 1 (a loss). It returns the
// updated counters and the calculated win_rate.
//
// Route: POST /words/{id}/register  (registered in NewWords).
// Auth:  the auth middleware has already established the session (web cookie or
//
//	mobile bearer token); we additionally require the row's created_by to
//	match the caller, so a user can only affect their own words.
func (m *Words) registerResult(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		functions.JSONError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var req registerReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		functions.JSONError(w, http.StatusBadRequest, "invalid request")
		return
	}
	success := req.Result == "success" || req.Result == "win" || req.Result == "true" || req.Result == "1"
	if req.Success != nil {
		success = *req.Success
	}

	s := cache.SessionFromContext(r.Context())
	if s == nil || s.UserID <= 0 {
		functions.JSONError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	db, err := pgdb.GetInstance()
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "database unavailable")
		return
	}

	// One atomic, owner-scoped update. The counter column is chosen by the caller
	// (a fixed identifier, never user input, so no injection surface).
	counter := "fail"
	if success {
		counter = "success"
	}
	query := "UPDATE words SET tries = COALESCE(tries,0) + 1, " + counter + " = COALESCE(" + counter + ",0) + 1, updated = now() " +
		"WHERE id = $1 AND created_by = $2 " +
		"RETURNING id, word, tries, success, fail, " +
		"CASE WHEN (COALESCE(success,0) + COALESCE(fail,0)) > 0 THEN round(COALESCE(success,0)::numeric / (COALESCE(success,0) + COALESCE(fail,0)), 4) ELSE 0 END AS win_rate"

	row, err := db.GetOne(query, id, s.UserID)
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if row == nil {
		// No such word, or it isn't the caller's.
		functions.JSONError(w, http.StatusNotFound, "word not found")
		return
	}

	functions.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"success": success,
		"word":    row,
	})
}
