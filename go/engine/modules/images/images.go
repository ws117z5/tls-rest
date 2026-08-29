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

	"tls-rest/go/engine/controllers/db/cache"
	"tls-rest/go/engine/controllers/db/pgdb"
	. "tls-rest/go/engine/controllers/field"
	"tls-rest/go/engine/controllers/functions"
	"tls-rest/go/engine/controllers/module"
	. "tls-rest/go/engine/controllers/module"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

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
		functions.JSONError(w, http.StatusBadRequest, "invalid multipart form: "+err.Error())
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		functions.JSONError(w, http.StatusBadRequest, "missing 'image' file: "+err.Error())
		return
	}
	defer file.Close()

	raw, err := io.ReadAll(io.LimitReader(file, maxUpload))
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "failed to read upload: "+err.Error())
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
		functions.JSONError(w, http.StatusBadRequest, "image processing failed: "+err.Error())
		return
	}

	guid := uuid.NewString()

	// Record the uploader so images can be listed/filtered by user.
	uploaderID := 0
	if s := cache.SessionFromContext(r.Context()); s != nil {
		uploaderID = s.UserID
	}

	db, err := pgdb.GetInstance()
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "database unavailable")
		return
	}
	id, err := db.InsertRow("images", map[string]interface{}{
		"uuid":       guid,
		"module":     moduleName,
		"field":      field,
		"record_id":  recordID,
		"filename":   header.Filename,
		"mime_type":  mimeType,
		"access":     access,
		"data":       data,
		"created_by": uploaderID,
	})
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Image metadata (EXIF/dimensions/original name captured client-side from the
	// pre-conversion file) is stored as JSONB. We enrich it with the stored byte
	// size and set it in a second statement so the text is cast to jsonb.
	if meta := buildMetadata(r.FormValue("metadata"), header.Filename, mimeType, len(data)); meta != "" {
		// Best-effort: the image is already stored; ignore a metadata failure.
		_, _ = db.Exec("UPDATE images SET metadata = $1::jsonb WHERE id = $2", meta, id)
	}

	imageCache.Set(guid, cachedImage{Id: id, Data: data, MimeType: mimeType})

	name := guid
	if ext := imageExt(header.Filename, mimeType); ext != "" {
		name = guid + "." + ext
	}

	functions.WriteJSON(w, http.StatusOK, map[string]interface{}{
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

// --- module definition -------------------------------------------------------

type Images struct {
	*ModuleAbstract[interface{}]
}

// fieldset defines the image metadata columns. id, uuid, access and created are
// system fields added automatically by Initialize().
func (m *Images) fieldset() []Field {
	return []Field{
		// Thumbnail: the image is served at /image/<uuid>, so a read-only IMAGE
		// field aliased to the uuid column renders the picture in list/view.
		NewField("preview", TYPE_IMAGE, false).WithLabel("Preview").WithSQL("uuid").AsReadOnly().NonSortable(),
		NewField("module", TYPE_STRING, false).WithLabel("Module"),
		NewField("field", TYPE_STRING, false).WithLabel("Field"),
		NewField("record_id", TYPE_INT, false).WithLabel("Record"),
		NewField("filename", TYPE_STRING, false).WithLabel("Filename"),
		NewField("mime_type", TYPE_STRING, false).WithLabel("Type"),
		// Uploader: virtual username resolved from the created_by user id.
		NewField("uploaded_by", TYPE_STRING, false).
			WithLabel("Uploaded by").
			AsVirtual().AsReadOnly().NonSortable().
			WithSQL("(SELECT user_name FROM users u WHERE u.id = images.created_by)"),
	}
}

func readOnly(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "access_log is read-only", http.StatusMethodNotAllowed)
}

// filters declares the list-mode filter bar for the images module (admin area):
// filter by filename, owning module/field, and mime type.
func (m *Images) filters() *Filedset {
	return NewFieldset(
		NewFilter("filename", TYPE_STRING).WithLabel("Filename").Contains(),
		NewFilter("module", TYPE_STRING).WithLabel("Module").Contains(),
		NewFilter("field", TYPE_STRING).WithLabel("Field").Contains(),
		NewFilter("mime_type", TYPE_STRING).WithLabel("Type").Contains(),
		// Uploader: type a username (substring). Matches images whose created_by
		// resolves to a user whose name ILIKEs the value.
		NewFilter("uploaded_by", TYPE_STRING).WithLabel("Uploaded by").
			Contains().
			WithSQLWhere("created_by IN (SELECT id FROM users WHERE user_name ILIKE %s)"),
	)
}

func NewImages() *Images {
	m := &Images{
		ModuleAbstract: &ModuleAbstract[interface{}]{
			ID:                   "images",
			Name:                 "Images",
			Submenu:              "engine",
			Rights:               make(map[int]int),
			DefaultPermission:    PERMISSION_READ,
			DefaultPermissionSet: true,
			OmitSystemFields:     []string{"updated"},
			HiddenModes:          []string{"edit"}, // no edit button; create (upload) stays
			Overrides: HandlerOverrides{
				Edit: readOnly,
			},
		},
	}
	m.ModuleAbstract.Fields = m.fieldset()
	m.ModuleAbstract.Filters = m.filters()

	// The module owns its binary endpoints as custom routes (absolute paths).
	m.ModuleAbstract.CustomRoutes = []CustomRoute{
		{Path: "/api/images/process", Methods: []string{"POST"}, Handler: Process, Absolute: true},
		{Path: "/image/{ref}", Methods: []string{"GET"}, Handler: ServeByRef, Absolute: true},
	}
	return m
}

func Init() {
	NewImages().Initialize("images")
}
