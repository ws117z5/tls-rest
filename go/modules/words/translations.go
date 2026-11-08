package words

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"tls-rest/go/engine/controllers/db/cache"
	"tls-rest/go/engine/controllers/db/pgdb"
	"tls-rest/go/engine/controllers/functions"

	"github.com/gorilla/mux"
)

type saveTranslationsReq struct {
	Translations []string `json:"translations"`
}

// saveTranslations handles POST /words/{id}/translations: replaces this
// word's stored translation candidates with the given list — the create
// flow's own lookup result (go/modules/words/lookup.go), submitted once the
// word itself exists and has an id. Replace-all (delete + insert), same as a
// fresh lookup result always being the full set, never an incremental patch.
//
// Route: POST /words/{id}/translations  (registered in NewWords).
// Auth: owner-scoped like registerResult — a user may only set translations
// on their own words.
func (m *Words) saveTranslations(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		functions.JSONError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var req saveTranslationsReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		functions.JSONError(w, http.StatusBadRequest, "invalid request")
		return
	}

	s := cache.SessionFromContext(r.Context())
	if s == nil || s.UserID <= 0 {
		functions.JSONError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	db, err := pgdb.GetInstanceCtx(r.Context())
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "database unavailable")
		return
	}

	owned, err := db.GetOne("SELECT id FROM words WHERE id = $1 AND created_by = $2", id, s.UserID)
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if owned == nil {
		functions.JSONError(w, http.StatusNotFound, "word not found")
		return
	}

	if _, err := db.Exec("DELETE FROM word_translations WHERE word_id = $1", id); err != nil {
		functions.JSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for _, text := range req.Translations {
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		if _, err := db.Exec("INSERT INTO word_translations (word_id, text) VALUES ($1, $2)", id, text); err != nil {
			functions.JSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	functions.WriteJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}
