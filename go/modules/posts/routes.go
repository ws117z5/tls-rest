package posts

import (
	"net/http"

	"tls-rest/go/engine/controllers/db/pgdb"
	"tls-rest/go/engine/controllers/functions"

	"github.com/gorilla/mux"
)

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// viewByRef handles GET /posts/<ref>. A numeric ref is the post id, same as always. Anything else is looked up as a
// slug and rewritten to its id before delegating to the module's normal View, so rights/visibility are enforced
// exactly as for an id lookup — this only translates which row is being asked for, never who may see it.
func viewByRef(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	ref := vars["ref"]
	if !isAllDigits(ref) {
		if db, err := pgdb.GetInstanceCtx(r.Context()); err == nil {
			if row, e := db.GetOne("SELECT id FROM posts WHERE slug = $1", ref); e == nil && row != nil {
				ref = functions.Coerce[string](row["id"])
			}
		}
	}
	vars["id"] = ref
	Module.View(w, r)
}
