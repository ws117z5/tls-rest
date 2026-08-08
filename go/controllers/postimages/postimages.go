package postimages

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"tls-rest/go/lib/db/cache"
	pgdb "tls-rest/go/lib/db/pgdb"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

// PostImage is an image attached to a post, stored in the database.
//
// Requires a table such as:
//
//	CREATE TABLE post_images (
//	    id         BIGSERIAL PRIMARY KEY,
//	    uuid       TEXT NOT NULL,
//	    post_id    BIGINT,
//	    filename   TEXT,
//	    mime_type  TEXT,
//	    data       BYTEA NOT NULL,
//	    created    TIMESTAMPTZ NOT NULL DEFAULT now()
//	);
type PostImage struct {
	tableName struct{} `pg:"post_images"`

	Id       int64  `pg:"id,pk" json:"id"`
	Uuid     string `pg:"uuid" json:"uuid"`
	PostId   int64  `pg:"post_id,use_zero" json:"post_id"`
	Filename string `pg:"filename" json:"filename"`
	MimeType string `pg:"mime_type" json:"mime_type"`
	Data     []byte `pg:"data" json:"-"`
}

const maxUpload = 16 << 20 // 16 MiB

// cachedImage is what we keep in memory for a served image.
type cachedImage struct {
	Data     []byte
	MimeType string
}

// imageCache serves image bytes from memory, loading from the database on a
// miss (the getter). Memory is bounded: entries expire after imageCacheTTL and
// total cached bytes are capped at imageCacheMaxBytes with LRU eviction.
const (
	imageCacheTTL      = 10 * time.Minute
	imageCacheMaxBytes = 64 << 20 // 64 MiB
)

var imageCache = cache.NewCache[cachedImage](loadImageFromDB, nil).
	WithTTL(imageCacheTTL).
	WithMaxBytes(imageCacheMaxBytes, func(ci cachedImage) int64 { return int64(len(ci.Data)) })

// loadImageFromDB is the cache getter: it fetches an image's bytes by id.
func loadImageFromDB(key string) (cachedImage, error) {
	id, err := strconv.ParseInt(key, 10, 64)
	if err != nil {
		return cachedImage{}, err
	}

	db, err := pgdb.GetInstance()
	if err != nil {
		return cachedImage{}, err
	}

	img := &PostImage{}
	if err := db.Model(img).Where("id = ?", id).Select(); err != nil {
		return cachedImage{}, err
	}

	return cachedImage{Data: img.Data, MimeType: img.MimeType}, nil
}

// Upload stores an uploaded image in the database and returns its metadata
// (including the generated id used as [img]<id>[/img] in a post body).
func Upload(w http.ResponseWriter, r *http.Request) {
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

	data, err := io.ReadAll(io.LimitReader(file, maxUpload))
	if err != nil {
		http.Error(w, "failed to read upload: "+err.Error(), http.StatusInternalServerError)
		return
	}

	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = http.DetectContentType(data)
	}

	var postID int64
	if v := r.FormValue("post_id"); v != "" {
		postID, _ = strconv.ParseInt(v, 10, 64)
	}

	img := &PostImage{
		Uuid:     uuid.NewString(),
		PostId:   postID,
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
		log.Printf("postimages Upload: insert failed: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Prime the cache so the first request for this image avoids a DB round-trip.
	imageCache.Set(strconv.FormatInt(img.Id, 10), cachedImage{Data: data, MimeType: mimeType})

	writeJSON(w, map[string]interface{}{
		"id":        img.Id,
		"uuid":      img.Uuid,
		"post_id":   img.PostId,
		"filename":  img.Filename,
		"mime_type": img.MimeType,
	})
}

// Serve returns the raw image bytes for <img src="/api/posts/images/{id}">,
// reading from the in-memory cache (which loads from the DB on a miss).
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

// List returns image metadata (no bytes), optionally filtered by post_id.
func List(w http.ResponseWriter, r *http.Request) {
	db, err := pgdb.GetInstance()
	if err != nil {
		http.Error(w, "database unavailable", http.StatusInternalServerError)
		return
	}

	var images []PostImage
	q := db.Model(&images).Column("id", "uuid", "post_id", "filename", "mime_type")
	if v := r.URL.Query().Get("post_id"); v != "" {
		if postID, perr := strconv.ParseInt(v, 10, 64); perr == nil {
			q = q.Where("post_id = ?", postID)
		}
	}
	if err := q.Order("id ASC").Select(); err != nil {
		log.Printf("postimages List: select failed: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, images)
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("postimages: json encode failed: %v", err)
	}
}
