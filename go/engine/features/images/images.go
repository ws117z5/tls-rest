// Package images is the images module: one module that owns everything about
// images — the metadata table (via the fieldset engine) AND the binary
// upload/serve endpoints (via the module's CustomRoutes). There is no separate
// "feature" package; the two things the generic CRUD can't do (accept a
// multipart upload, stream raw bytes) are registered as extra routes on this
// module.
//
//	GET/POST /images, /images/{id}   standard CRUD over image metadata (access-filtered)
//	POST     /api/images/process     upload + preprocess + store  (CustomRoute)
//	GET      /image/{guid}.{ext}     serve bytes, access-controlled (CustomRoute)
//
// Access: serving asks the engine (module.CanViewRecord) whether the request may
// read the specific image record — the same row-level rule every module uses. An
// image's access is defaulted at upload from the record it's attached to
// (module + record_id) and overridable with an explicit level.
//
// The bytes live in a `data BYTEA` column that the fieldset engine does not
// create (it has no binary field type), so create/migrate the table explicitly:
//
//	CREATE TABLE images (
//	    id BIGSERIAL PRIMARY KEY, uuid TEXT NOT NULL UNIQUE,
//	    module TEXT, field TEXT, record_id BIGINT,
//	    filename TEXT, mime_type TEXT,
//	    access INT NOT NULL DEFAULT 0, data BYTEA NOT NULL,
//	    created TIMESTAMPTZ NOT NULL DEFAULT now()
//	);
package images

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	module "tls-rest/go/engine"
	"tls-rest/go/lib"
	"tls-rest/go/lib/db/cache"
	"tls-rest/go/lib/db/pgdb"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

// --- module definition -------------------------------------------------------

type Images struct {
	*module.ModuleAbstract[interface{}]
}

func NewImages() *Images {
	m := &Images{
		ModuleAbstract: &module.ModuleAbstract[interface{}]{
			ID:                   "images",
			Name:                 "Images",
			Rights:               make(map[int]int),
			DefaultPermission:    1, // PERMISSION_READ
			DefaultPermissionSet: true,
		},
	}
	m.ModuleAbstract.Fields = m.fieldset()

	// The module owns its binary endpoints as custom routes (absolute paths).
	m.ModuleAbstract.CustomRoutes = []module.CustomRoute{
		{Path: "/api/images/process", Methods: []string{"POST"}, Handler: Process, Absolute: true},
		{Path: "/image/{ref}", Methods: []string{"GET"}, Handler: ServeByRef, Absolute: true},
	}
	return m
}

// fieldset defines the image metadata columns. id, uuid, access and created are
// system fields added automatically by Initialize().
func (m *Images) fieldset() []module.Field {
	return []module.Field{
		module.NewField("module", module.TYPE_STRING, false).WithLabel("Module"),
		module.NewField("field", module.TYPE_STRING, false).WithLabel("Field"),
		module.NewField("record_id", module.TYPE_INT, false).WithLabel("Record"),
		module.NewField("filename", module.TYPE_STRING, false).WithLabel("Filename"),
		module.NewField("mime_type", module.TYPE_STRING, false).WithLabel("Type"),
	}
}

func init() {
	NewImages().Initialize("images")
}

// --- binary upload / serve ---------------------------------------------------

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

type cachedImage struct {
	Id       int64
	Data     []byte
	MimeType string
}

const (
	imageCacheTTL      = 10 * time.Minute
	imageCacheMaxBytes = 64 << 20 // 64 MiB
)

var imageCache = cache.NewCache[cachedImage](loadImage, nil).
	WithTTL(imageCacheTTL).
	WithMaxBytes(imageCacheMaxBytes, func(ci cachedImage) int64 { return int64(len(ci.Data)) })

// thumbCache holds generated 80x80 preview JPEGs, keyed by the same ref. Its
// getter builds the thumbnail from the full image (imageCache) on a miss. On an
// undecodable source (e.g. HEIC) it stores an empty entry so ServeByRef falls
// back to the original bytes without retrying every request.
var thumbCache = cache.NewCache[cachedImage](loadThumbnail, nil).
	WithTTL(imageCacheTTL).
	WithMaxBytes(imageCacheMaxBytes, func(ci cachedImage) int64 { return int64(len(ci.Data)) })

func loadThumbnail(ref string) (cachedImage, error) {
	full, err := imageCache.Get(ref)
	if err != nil || full == nil || len(full.Data) == 0 {
		return cachedImage{}, err
	}
	data, mime, terr := makeThumbnail(full.Data, previewSize)
	if terr != nil {
		// Undecodable source: cache an empty thumb so we don't retry; caller
		// serves the original bytes instead.
		return cachedImage{Id: full.Id}, nil
	}
	return cachedImage{Id: full.Id, Data: data, MimeType: mime}, nil
}

// loadImage is the cache getter: fetches an image's id + bytes by its guid
// (uuid) or, for legacy references, by numeric id.
func loadImage(ref string) (cachedImage, error) {
	db, err := pgdb.GetInstance()
	if err != nil {
		return cachedImage{}, err
	}

	const cols = "id, data, mime_type"
	var row map[string]interface{}
	if isAllDigits(ref) {
		id, _ := strconv.ParseInt(ref, 10, 64)
		row, err = db.GetOne("SELECT "+cols+" FROM images WHERE id = $1", id)
	} else {
		row, err = db.GetOne("SELECT "+cols+" FROM images WHERE uuid = $1", ref)
	}
	if err != nil {
		return cachedImage{}, err
	}
	if row == nil {
		return cachedImage{}, nil // not found -> empty (serve turns this into 404)
	}

	return cachedImage{
		Id:       pgdb.Coerce[int64](row["id"]),
		Data:     pgdb.Coerce[[]byte](row["data"]),
		MimeType: pgdb.Coerce[string](row["mime_type"]),
	}, nil
}

// Process handles POST /api/images/process: upload + preprocess + store.
func Process(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(maxUpload); err != nil {
		lib.JSONError(w, http.StatusBadRequest, "invalid multipart form: "+err.Error())
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		lib.JSONError(w, http.StatusBadRequest, "missing 'image' file: "+err.Error())
		return
	}
	defer file.Close()

	raw, err := io.ReadAll(io.LimitReader(file, maxUpload))
	if err != nil {
		lib.JSONError(w, http.StatusInternalServerError, "failed to read upload: "+err.Error())
		return
	}

	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = http.DetectContentType(raw)
	}

	moduleName := r.FormValue("module")
	field := r.FormValue("field")

	var recordID int64
	if v := r.FormValue("record_id"); v != "" {
		recordID, _ = strconv.ParseInt(v, 10, 64)
	}

	// Effective access: explicit override wins; else inherit the owning record's
	// access at upload; else public (0).
	access := 0
	if v := r.FormValue("access"); v != "" {
		if a, aerr := strconv.Atoi(v); aerr == nil {
			access = a
		}
	} else if moduleName != "" && recordID != 0 {
		access = recordAccess(moduleName, recordID)
	}

	data, mimeType, err := processorFor(moduleName, field)(moduleName, field, raw, mimeType)
	if err != nil {
		lib.JSONError(w, http.StatusBadRequest, "image processing failed: "+err.Error())
		return
	}

	guid := uuid.NewString()

	db, err := pgdb.GetInstance()
	if err != nil {
		lib.JSONError(w, http.StatusInternalServerError, "database unavailable")
		return
	}
	id, err := db.InsertRow("images", map[string]interface{}{
		"uuid":      guid,
		"module":    moduleName,
		"field":     field,
		"record_id": recordID,
		"filename":  header.Filename,
		"mime_type": mimeType,
		"access":    access,
		"data":      data,
	})
	if err != nil {
		lib.JSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	imageCache.Set(guid, cachedImage{Id: id, Data: data, MimeType: mimeType})

	name := guid
	if ext := imageExt(header.Filename, mimeType); ext != "" {
		name = guid + "." + ext
	}

	lib.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"id":        id,
		"uuid":      guid,
		"module":    moduleName,
		"field":     field,
		"filename":  header.Filename,
		"mime_type": mimeType,
		"url":       "/image/" + name,
	})
}

// ServeByRef handles GET /image/{guid}.{ext}: reads bytes from cache (DB on a
// miss) and defers the access decision to the engine (module.CanViewRecord).
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

	if ok, err := module.CanViewRecord(r, "images", ci.Id); err != nil || !ok {
		http.NotFound(w, r) // 404, not 403: don't reveal restricted images exist
		return
	}

	// Preview mode: serve the compressed 80x80 content-aware thumbnail. Access
	// was already checked above (the thumbnail is the same record). Falls back to
	// the original bytes if the source couldn't be decoded (e.g. HEIC).
	if r.URL.Query().Has("preview") {
		if t, terr := thumbCache.Get(ref); terr == nil && t != nil && len(t.Data) > 0 {
			w.Header().Set("Content-Type", t.MimeType)
			w.Header().Set("Cache-Control", "private, max-age=300")
			w.Write(t.Data)
			return
		}
	}

	if ci.MimeType != "" {
		w.Header().Set("Content-Type", ci.MimeType)
	}
	w.Header().Set("Cache-Control", "private, max-age=0, must-revalidate")
	w.Write(ci.Data)
}

// recordAccess returns the access level of an owning record, or 0 if unknown.
func recordAccess(moduleName string, recordID int64) int {
	if !isValidIdent(moduleName) {
		return 0
	}
	db, err := pgdb.GetInstance()
	if err != nil {
		return 0
	}
	row, err := db.GetOne("SELECT access FROM "+db.Quote(moduleName)+" WHERE id = $1", recordID)
	if err != nil || row == nil || row["access"] == nil {
		return 0
	}
	return pgdb.Coerce[int](row["access"])
}

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
