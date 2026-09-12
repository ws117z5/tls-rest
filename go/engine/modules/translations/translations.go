// Package translations backs the site's i18n: UI strings keyed by their
// English text, resolved per locale from the translations table. A key is
// seeded with its English default the first time anyone asks for it in any
// locale; admins fix up non-English rows through the standalone CRUD module
// here. Missing translations just fall back to English until edited.
//
//	POST /api/i18n/resolve  {locale, strings: {key: englishDefault}}  -> {key: text}
package translations

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"tls-rest/go/engine/controllers/db/cache"
	"tls-rest/go/engine/controllers/db/pgdb"
	"tls-rest/go/engine/controllers/field"
	"tls-rest/go/engine/controllers/functions"
	"tls-rest/go/engine/controllers/module"

	"github.com/gorilla/mux"
)

// Module is the admin-only CRUD view over the raw translations table. The
// table itself is created from this fieldset by the engine (init/sql pins the
// types and adds the (key, locale) uniqueness).
var Module = &module.ModuleAbstract[interface{}]{
	ID:      "translations",
	Name:    "Translations",
	Icon:    "config",
	Submenu: "engine",
	Fields: []field.Field{
		field.NewField("key", field.TYPE_STRING, true).WithLabel("Key"),
		field.NewField("locale", field.TYPE_STRING, true).WithLabel("Locale"),
		field.NewField("value", field.TYPE_TEXT, true).WithLabel("Value"),
	},
	DefaultPermission:    module.PERMISSION_DENY,
	DefaultPermissionSet: true,
	Rights:               make(map[int]int),
}

func Init() {
	Module.Initialize("translations")

	module.RegisterEndpointPrefix("/api/i18n")
	module.AddRouteRegistrar(func(r *mux.Router) {
		r.HandleFunc("/api/i18n/resolve", handleResolve).Methods("POST")
	})
}

var errMiss = errors.New("translations: not cached")

// stringCache holds resolved (locale,key) -> value lookups. Get on a miss
// loads from the DB; Set seeds a new row (used only for never-before-seen keys).
var stringCache = cache.NewCache[string](loadFromDB, saveToDB).WithTTL(time.Hour)

func cacheKey(locale, key string) string { return locale + "\x00" + key }

func loadFromDB(ck string) (string, error) {
	locale, key, _ := strings.Cut(ck, "\x00")
	db, err := pgdb.GetInstance()
	if err != nil {
		return "", err
	}
	row, err := db.GetOne(`SELECT value FROM translations WHERE key = $1 AND locale = $2`, key, locale)
	if err != nil || row == nil {
		return "", errMiss
	}
	return functions.Coerce[string](row["value"]), nil
}

func saveToDB(ck string, value string) error {
	locale, key, _ := strings.Cut(ck, "\x00")
	db, err := pgdb.GetInstance()
	if err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO translations (key, locale, value) VALUES ($1, $2, $3)
		ON CONFLICT (key, locale) DO NOTHING`, key, locale, value)
	return err
}

// Resolve returns key's text for locale, seeding the English default the
// first time this key is ever seen in any locale. A locale with no row for
// key yet just falls back to the English one.
func Resolve(key, locale, def string) string {
	if key == "" {
		return def
	}
	if locale = strings.ToLower(strings.TrimSpace(locale)); locale != "" && locale != "en" {
		if v, err := stringCache.Get(cacheKey(locale, key)); err == nil {
			return *v
		}
	}
	if v, err := stringCache.Get(cacheKey("en", key)); err == nil {
		return *v
	}
	stringCache.Set(cacheKey("en", key), def)
	return def
}

type resolveBody struct {
	Locale  string            `json:"locale"`
	Strings map[string]string `json:"strings"`
}

// handleResolve is public (no session required) — translated text must work
// for anonymous visitors too.
func handleResolve(w http.ResponseWriter, r *http.Request) {
	var in resolveBody
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || len(in.Strings) == 0 {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	out := make(map[string]string, len(in.Strings))
	for key, def := range in.Strings {
		out[key] = Resolve(key, in.Locale, def)
	}
	writeJSON(w, http.StatusOK, out)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
