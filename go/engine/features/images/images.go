// Package images is the generic, module-agnostic image controller. It is its own
// module: images are uploaded once, stored in the database, and referenced from
// content (e.g. post bodies) by a stable URL:
//
//	POST /api/images/process     upload + preprocess + store, returns {id,uuid,url}
//	GET  /image/{guid}.{ext}      serve the stored bytes (access-controlled)
//
// # Access control
//
// An image records which record it belongs to (module + record_id). On serve,
// its effective access level is:
//
//   - the image's own access column, if set (a per-image override); otherwise
//   - the access level of the owning record (inherited); otherwise
//   - 0 (public) when there is nothing to inherit.
//
// The viewer may see the image when they are an admin or their session access
// level is >= the effective access level. Denied requests return 404 so the
// existence of a restricted image is not revealed. Because serving is gated, the
// response is marked private (never cached by shared proxies).
package images

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"tls-rest/go/lib/db/cache"
	pgdb "tls-rest/go/lib/db/pgdb"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

// Image is one stored image. Requires a table such as:
//
//	CREATE TABLE images (
//	    id        BIGSERIAL PRIMARY KEY,
//	    uuid      TEXT NOT NULL UNIQUE,
//	    module    TEXT,           -- owning record's module
//	    field     TEXT,           -- owning field (context for the preprocessor)
//	    record_id BIGINT,         -- owning record id (0 = unattached)
//	    filename  TEXT,
//	    mime_type TEXT,
//	    access    INT,            -- NULL = inherit from (module, record_id); set = override
//	    data      BYTEA NOT NULL,
//	    created   TIMESTAMPTZ NOT NULL DEFAULT now()
//	);
type Image struct {
	Id       int64
	Uuid     string
	Module   string
	Field    string
	RecordId int64
	Filename string
	MimeType string
	Access   *int
	Data     []byte
}

const maxUpload = 16 << 20 // 16 MiB

// Preprocessor transforms uploaded bytes before they are stored.
type Preprocessor func(module, field string, data []byte, mimeType string) (out []byte, outMime string, err error)

func identityProcessor(_, _ string, data []byte, mimeType string) ([]byte, string, error) {
	return data, mimeType, nil
}

var (
	processors       = map[string]Preprocessor{}
	defaultProcessor = Preprocessor(identityProcessor)
	processorKey     = func(module, field string) string { return module + "." + field }
)

// RegisterProcessor sets a custom preprocessor for a specific (module, field).
func RegisterProcessor(module, field string, fn Preprocessor) {
	if fn != nil {
		processors[processorKey(module, field)] = fn
	}
}

// SetDefaultProcessor overrides the fallback preprocessor.
func SetDefaultProcessor(fn Preprocessor) {
	if fn != nil {
		defaultProcessor = fn
	}
}

func processorFor(module, field string) Preprocessor {
	if fn, ok := processors[processorKey(module, field)]; ok {
		return fn
	}
	return defaultProcessor
}

// --- in-memory cache (bytes + access metadata, loaded from the DB on a miss) ---

type cachedImage struct {
	Data     []byte
	MimeType string
	Module   string
	RecordId int64
	Access   *int // per-image override; nil = inherit from owning record
}

const (
	imageCacheTTL      = 10 * time.Minute
	imageCacheMaxBytes = 64 << 20 // 64 MiB
)

var imageCache = cache.NewCache[cachedImage](loadImage, nil).
	WithTTL(imageCacheTTL).
	WithMaxBytes(imageCacheMaxBytes, func(ci cachedImage) int64 { return int64(len(ci.Data)) })

// loadImage is the cache getter: it fetches an image by its guid (uuid) or, for
// backwards compatibility, by a numeric id.
func loadImage(ref string) (cachedImage, error) {
	db, err := pgdb.GetInstance()
	if err != nil {
		return cachedImage{}, err
	}

	const cols = "data, mime_type, module, record_id, access"
	var rows []map[string]interface{}
	if isAllDigits(ref) {
		id, _ := strconv.ParseInt(ref, 10, 64)
		rows, err = db.RQuery("SELECT "+cols+" FROM images WHERE id = $1", id)
	} else {
		rows, err = db.RQuery("SELECT "+cols+" FROM images WHERE uuid = $1", ref)
	}
	if err != nil {
		return cachedImage{}, err
	}
	if len(rows) == 0 {
		return cachedImage{}, fmt.Errorf("image %s not found", ref)
	}

	row := rows[0]
	var access *int
	if row["access"] != nil {
		a := int(pgdb.AsInt64(row["access"]))
		access = &a
	}
	return cachedImage{
		Data:     pgdb.AsBytes(row["data"]),
		MimeType: pgdb.AsString(row["mime_type"]),
		Module:   pgdb.AsString(row["module"]),
		RecordId: pgdb.AsInt64(row["record_id"]),
		Access:   access,
	}, nil
}

// effectiveAccess resolves an image's access level: the per-image override if
// set, otherwise the owning record's access, otherwise 0 (public).
func effectiveAccess(ci cachedImage) int {
	if ci.Access != nil {
		return *ci.Access
	}
	if ci.Module != "" && ci.RecordId != 0 && isValidIdent(ci.Module) {
		if db, err := pgdb.GetInstance(); err == nil {
			rows, err := db.RQuery("SELECT access FROM "+db.Quote(ci.Module)+" WHERE id = $1", ci.RecordId)
			if err == nil && len(rows) > 0 && rows[0]["access"] != nil {
				return int(pgdb.AsInt64(rows[0]["access"]))
			}
		}
	}
	return 0
}

// canView reports whether the request's session may view the image.
func canView(r *http.Request, ci cachedImage) bool {
	s := cache.SessionFromContext(r.Context())
	if s != nil && s.IsAdmin {
		return true
	}
	level := 0
	if s != nil {
		level = s.AccessLevel
	}
	return effectiveAccess(ci) <= level
}

// Process handles POST /api/images/process: upload + preprocess + store. It
// expects a multipart form with an "image" file plus "module"/"field" (so the
// right preprocessor runs) and optionally "record_id" (the owning record) and
// "access" (a per-image override level). Returns the new image's metadata and
// its /image/{guid}.{ext} URL.
func Process(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(maxUpload); err != nil {
		http.Error(w, "invalid multipart form: "+err.Error(), http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "missing 'image' file: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	raw, err := io.ReadAll(io.LimitReader(file, maxUpload))
	if err != nil {
		http.Error(w, "failed to read upload: "+err.Error(), http.StatusInternalServerError)
		return
	}

	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = http.DetectContentType(raw)
	}

	module := r.FormValue("module")
	field := r.FormValue("field")

	var recordID int64
	if v := r.FormValue("record_id"); v != "" {
		recordID, _ = strconv.ParseInt(v, 10, 64)
	}

	// Optional per-image access override. Absent => NULL => inherit.
	var access interface{}
	var accessPtr *int
	if v := r.FormValue("access"); v != "" {
		if a, aerr := strconv.Atoi(v); aerr == nil {
			access = a
			accessPtr = &a
		}
	}

	// Run the (module, field) preprocessor (or the default) before saving.
	data, mimeType, err := processorFor(module, field)(module, field, raw, mimeType)
	if err != nil {
		http.Error(w, "image processing failed: "+err.Error(), http.StatusBadRequest)
		return
	}

	guid := uuid.NewString()

	db, err := pgdb.GetInstance()
	if err != nil {
		http.Error(w, "database unavailable", http.StatusInternalServerError)
		return
	}
	id, err := db.InsertRow("images", map[string]interface{}{
		"uuid":      guid,
		"module":    module,
		"field":     field,
		"record_id": recordID,
		"filename":  header.Filename,
		"mime_type": mimeType,
		"access":    access,
		"data":      data,
	})
	if err != nil {
		log.Printf("images Process: insert failed: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Prime the cache so the first display avoids a DB round-trip.
	imageCache.Set(guid, cachedImage{
		Data: data, MimeType: mimeType, Module: module, RecordId: recordID, Access: accessPtr,
	})

	name := guid
	if ext := imageExt(header.Filename, mimeType); ext != "" {
		name = guid + "." + ext
	}

	writeJSON(w, map[string]interface{}{
		"id":        id,
		"uuid":      guid,
		"module":    module,
		"field":     field,
		"filename":  header.Filename,
		"mime_type": mimeType,
		"url":       "/image/" + name,
	})
}

// ServeByRef handles GET /image/{guid}.{ext} (the extension is cosmetic and
// ignored). It reads bytes from cache (loading from the DB on a miss) and
// enforces access before returning them.
func ServeByRef(w http.ResponseWriter, r *http.Request) {
	ref := mux.Vars(r)["ref"]
	if i := strings.LastIndexByte(ref, '.'); i >= 0 {
		ref = ref[:i]
	}
	if ref == "" {
		http.Error(w, "invalid image reference", http.StatusBadRequest)
		return
	}

	ci, err := imageCache.Get(ref)
	if err != nil || ci == nil || len(ci.Data) == 0 {
		http.NotFound(w, r)
		return
	}
	if !canView(r, *ci) {
		http.NotFound(w, r) // 404, not 403: don't reveal restricted images exist
		return
	}

	if ci.MimeType != "" {
		w.Header().Set("Content-Type", ci.MimeType)
	}
	// Access-controlled: must not be cached by shared proxies.
	w.Header().Set("Cache-Control", "private, max-age=0, must-revalidate")
	w.Write(ci.Data)
}

// imageExt returns a file extension for the image URL, preferring the uploaded
// filename's extension and falling back to the mime type.
func imageExt(filename, mime string) string {
	if i := strings.LastIndexByte(filename, '.'); i >= 0 && i < len(filename)-1 {
		return strings.ToLower(filename[i+1:])
	}
	switch mime {
	case "image/png":
		return "png"
	case "image/jpeg":
		return "jpg"
	case "image/gif":
		return "gif"
	case "image/webp":
		return "webp"
	case "image/svg+xml":
		return "svg"
	}
	return ""
}

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

// isValidIdent guards the module name used as a table identifier in the inherit
// query (defence in depth; module is server-supplied at upload).
func isValidIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, c := range s {
		switch {
		case c == '_':
		case c >= 'a' && c <= 'z':
		case c >= 'A' && c <= 'Z':
		case i > 0 && c >= '0' && c <= '9':
		default:
			return false
		}
	}
	return true
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("images: json encode failed: %v", err)
	}
}
