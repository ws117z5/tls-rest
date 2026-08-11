// Package images is the generic, module-agnostic image controller used by the
// Image field type. It exposes two endpoints:
//
//	POST /api/images/process   upload + preprocess + store, returns {id,uuid,...}
//	GET  /api/images/{id}       serve the stored bytes (cached)
//
// On upload the current module and field are supplied so the correct
// preprocessor runs before the bytes are saved (a per-(module,field) handler if
// one is registered, otherwise the default identity handler). After saving, the
// image has an id/uuid/filename which the caller stores in the module's Image
// field for later display. Bytes are stored in the database and cached in memory
// after upload/access.
package images

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	engine "tls-rest/go/engine"
	"tls-rest/go/lib/db/cache"
	pgdb "tls-rest/go/lib/db/pgdb"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

// init self-registers the image endpoints through the shared route-registrar
// seam, so route.go no longer hardcodes them. Uploading/processing is gated to
// authenticated users by the middleware; serving is public.
func init() {
	engine.AddRouteRegistrar(func(router *mux.Router) {
		router.HandleFunc("/api/images/process", Process).Methods("POST")
		router.HandleFunc("/api/images/{id}", Serve).Methods("GET")
	})
}

// Image is one stored image. Requires a table such as:
//
//	CREATE TABLE images (
//	    id        BIGSERIAL PRIMARY KEY,
//	    uuid      TEXT NOT NULL,
//	    module    TEXT,
//	    field     TEXT,
//	    filename  TEXT,
//	    mime_type TEXT,
//	    data      BYTEA NOT NULL,
//	    created   TIMESTAMPTZ NOT NULL DEFAULT now()
//	);
type Image struct {
	tableName struct{} `pg:"images"`

	Id       int64  `pg:"id,pk" json:"id"`
	Uuid     string `pg:"uuid" json:"uuid"`
	Module   string `pg:"module" json:"module"`
	Field    string `pg:"field" json:"field"`
	Filename string `pg:"filename" json:"filename"`
	MimeType string `pg:"mime_type" json:"mime_type"`
	Data     []byte `pg:"data" json:"-"`
}

const maxUpload = 16 << 20 // 16 MiB

// Preprocessor transforms uploaded bytes before they are stored. It receives the
// raw bytes and detected mime type and returns the bytes/mime to persist. The
// module and field are provided for context (e.g. different sizes per field).
type Preprocessor func(module, field string, data []byte, mimeType string) (out []byte, outMime string, err error)

// identityProcessor stores the upload unchanged.
func identityProcessor(_, _ string, data []byte, mimeType string) ([]byte, string, error) {
	return data, mimeType, nil
}

var (
	processors       = map[string]Preprocessor{}
	defaultProcessor = Preprocessor(identityProcessor)
	processorKey     = func(module, field string) string { return module + "." + field }
)

// RegisterProcessor sets a custom preprocessor for a specific (module, field).
// Modules register these in their init() to override the default handling.
func RegisterProcessor(module, field string, fn Preprocessor) {
	if fn != nil {
		processors[processorKey(module, field)] = fn
	}
}

// SetDefaultProcessor overrides the fallback preprocessor used when a
// (module, field) has no specific one registered.
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

// --- in-memory cache (bytes are loaded from the DB on a miss) ---

type cachedImage struct {
	Data     []byte
	MimeType string
}

const (
	imageCacheTTL      = 10 * time.Minute
	imageCacheMaxBytes = 64 << 20 // 64 MiB
)

var imageCache = cache.NewCache[cachedImage](loadImageFromDB, nil).
	WithTTL(imageCacheTTL).
	WithMaxBytes(imageCacheMaxBytes, func(ci cachedImage) int64 { return int64(len(ci.Data)) })

func loadImageFromDB(key string) (cachedImage, error) {
	id, err := strconv.ParseInt(key, 10, 64)
	if err != nil {
		return cachedImage{}, err
	}
	db, err := pgdb.GetInstance()
	if err != nil {
		return cachedImage{}, err
	}
	img := &Image{}
	if err := db.Model(img).Where("id = ?", id).Select(); err != nil {
		return cachedImage{}, err
	}
	return cachedImage{Data: img.Data, MimeType: img.MimeType}, nil
}

// Process handles POST /api/images/process. It expects a multipart form with an
// "image" file and "module"/"field" values so the right preprocessor runs, then
// stores the (possibly transformed) bytes and returns the new image's metadata.
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

	// Run the (module, field) preprocessor (or the default) before saving.
	data, mimeType, err := processorFor(module, field)(module, field, raw, mimeType)
	if err != nil {
		http.Error(w, "image processing failed: "+err.Error(), http.StatusBadRequest)
		return
	}

	img := &Image{
		Uuid:     uuid.NewString(),
		Module:   module,
		Field:    field,
		Filename: header.Filename,
		MimeType: mimeType,
		Data:     data,
	}

	db, err := pgdb.GetInstance()
	if err != nil {
		http.Error(w, "database unavailable", http.StatusInternalServerError)
		return
	}
	if _, err := db.Model(img).Insert(); err != nil {
		log.Printf("images Process: insert failed: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Prime the cache so the first display avoids a DB round-trip.
	imageCache.Set(strconv.FormatInt(img.Id, 10), cachedImage{Data: data, MimeType: mimeType})

	writeJSON(w, map[string]interface{}{
		"id":        img.Id,
		"uuid":      img.Uuid,
		"module":    img.Module,
		"field":     img.Field,
		"filename":  img.Filename,
		"mime_type": img.MimeType,
	})
}

// Serve handles GET /api/images/{id}, returning the raw bytes from cache
// (loading from the DB on a miss). Public: display is not gated (a public
// record's images must be viewable by anyone); upload is what requires auth.
func Serve(w http.ResponseWriter, r *http.Request) {
	idStr := mux.Vars(r)["id"]
	if _, err := strconv.ParseInt(idStr, 10, 64); err != nil {
		http.Error(w, "invalid image id", http.StatusBadRequest)
		return
	}

	img, err := imageCache.Get(idStr)
	if err != nil || img == nil || len(img.Data) == 0 {
		http.Error(w, "image not found", http.StatusNotFound)
		return
	}

	if img.MimeType != "" {
		w.Header().Set("Content-Type", img.MimeType)
	}
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Write(img.Data)
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("images: json encode failed: %v", err)
	}
}
