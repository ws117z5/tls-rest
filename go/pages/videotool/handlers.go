package videotool

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"

	"tls-rest/go/engine/controllers/functions"
	"tls-rest/go/engine/controllers/log"
)

// validateSourceURL rejects anything that isn't a well-formed http(s) URL resolving to a public address. This is
// the first of two independent layers against the yt-dlp subprocess loadFromURL feeds url into (the second is the
// "--" argv guard in ytdlp.go): a value can't be mistaken for a yt-dlp CLI flag if it's required to parse as an
// absolute http(s) URL, and resolving+checking the host blocks using yt-dlp's generic extractor as an SSRF proxy
// against internal/link-local targets (e.g. a cloud metadata endpoint).
func validateSourceURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return fmt.Errorf("must be a valid http(s) link")
	}
	ips, err := net.LookupIP(u.Hostname())
	if err != nil || len(ips) == 0 {
		return fmt.Errorf("could not resolve host")
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
			return fmt.Errorf("that host isn't allowed")
		}
	}
	return nil
}

func floatParam(r *http.Request, name string, def float64) float64 {
	v := r.URL.Query().Get(name)
	if v == "" {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return def
	}
	return f
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// startSSE writes SSE headers and returns a send func that JSON-encodes payload as one named event. Shared by
// loadFromURL and exportJob.
func startSSE(w http.ResponseWriter, r *http.Request) (send func(event string, payload any), ok bool) {
	flusher, isFlusher := w.(http.Flusher)
	if !isFlusher {
		functions.JSONError(w, http.StatusInternalServerError, "streaming unsupported")
		return nil, false
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	var mu sync.Mutex
	send = func(event string, payload any) {
		mu.Lock()
		defer mu.Unlock()
		select {
		case <-r.Context().Done():
			return
		default:
		}
		data, err := functions.WrapJSON(payload)
		if err != nil {
			return
		}
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
		flusher.Flush()
	}
	return send, true
}

// loadFromURL handles GET /api/video-tool/load?url=...: probes + downloads the source via yt-dlp into a fresh job,
// streaming progress as SSE, ending with "complete" {id, duration, title} once the source is ready to preview (see
// /api/video-tool/source/{id}), or "error".
func loadFromURL(w http.ResponseWriter, r *http.Request) {
	rawURL := strings.TrimSpace(r.URL.Query().Get("url"))
	send, ok := startSSE(w, r)
	if !ok {
		return
	}
	if rawURL == "" {
		send("error", map[string]string{"message": "url is required"})
		return
	}
	if err := validateSourceURL(rawURL); err != nil {
		send("error", map[string]string{"message": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Minute)
	defer cancel()

	send("status", map[string]string{"message": "Fetching video info…"})
	info, err := probeSource(ctx, rawURL)
	if err != nil {
		send("error", map[string]string{"message": "could not read source: " + err.Error()})
		return
	}
	if info.Duration > maxDurationSeconds {
		send("error", map[string]string{"message": fmt.Sprintf(
			"source is too long (%.0f min, limit %.0f min)", info.Duration/60, float64(maxDurationSeconds)/60)})
		return
	}

	id, j, err := newJob()
	if err != nil {
		send("error", map[string]string{"message": "could not allocate workspace"})
		return
	}

	send("status", map[string]string{"message": "Downloading source…"})
	srcPath, err := downloadSource(ctx, rawURL, j.dir, func(pct float64) {
		send("download_progress", map[string]float64{"percent": pct})
	})
	if err != nil {
		expireJob(id)
		send("error", map[string]string{"message": "download failed: " + err.Error()})
		return
	}

	j.mu.Lock()
	j.sourcePath = srcPath
	j.sourceDuration = info.Duration
	j.title = info.Title
	j.mu.Unlock()

	send("complete", map[string]any{"id": id, "duration": info.Duration, "title": info.Title})
}

// saveUpload streams a multipart "file" field into destDir/baseName.<ext>, capped at maxFilesizeBytes. The
// extension comes from the uploaded file's own name (sanitized), falling back to fallbackExt when absent/unsafe —
// it can only be known once the form is parsed, so the full path is built here, not by the caller.
func saveUpload(w http.ResponseWriter, r *http.Request, destDir, baseName, fallbackExt string) (path, origFilename string, err error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxFilesizeBytes)
	if err = r.ParseMultipartForm(32 << 20); err != nil {
		return "", "", fmt.Errorf("upload too large or invalid: %w", err)
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		return "", "", fmt.Errorf("file is required: %w", err)
	}
	defer file.Close()

	ext := sanitizeExt(filepath.Ext(header.Filename), fallbackExt)
	path = filepath.Join(destDir, baseName+"."+ext)
	out, err := os.Create(path)
	if err != nil {
		return "", "", err
	}
	defer out.Close()
	if _, err = io.Copy(out, file); err != nil {
		return "", "", fmt.Errorf("upload failed: %w", err)
	}
	return path, header.Filename, nil
}

// uploadVideo handles POST /api/video-tool/upload (multipart "file"): an alternative to loadFromURL for a local
// file. Responds with the same {id, duration, title} shape loadFromURL's "complete" event carries.
func uploadVideo(w http.ResponseWriter, r *http.Request) {
	id, j, err := newJob()
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "could not allocate workspace")
		return
	}

	destPath, origName, err := saveUpload(w, r, j.dir, "source", "mp4")
	if err != nil {
		expireJob(id)
		functions.JSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Minute)
	defer cancel()
	duration, err := ffprobeDuration(ctx, destPath)
	if err != nil {
		expireJob(id)
		functions.JSONError(w, http.StatusBadRequest, "unreadable video file: "+err.Error())
		return
	}
	if duration > maxDurationSeconds {
		expireJob(id)
		functions.JSONError(w, http.StatusBadRequest, fmt.Sprintf(
			"source is too long (%.0f min, limit %.0f min)", duration/60, float64(maxDurationSeconds)/60))
		return
	}

	title := sanitizeFilename(strings.TrimSuffix(origName, filepath.Ext(origName)))
	j.mu.Lock()
	j.sourcePath = destPath
	j.sourceDuration = duration
	j.title = title
	j.mu.Unlock()

	functions.WriteJSON(w, http.StatusOK, map[string]any{"id": id, "duration": duration, "title": title})
}

// uploadAudioTrack handles POST /api/video-tool/upload-audio?job=<id> (multipart "file"): attaches an extra audio
// track to an already-loaded job. Responds {duration}.
func uploadAudioTrack(w http.ResponseWriter, r *http.Request) {
	j, ok := getJob(r.URL.Query().Get("job"))
	if !ok {
		functions.JSONError(w, http.StatusNotFound, "job not found or expired")
		return
	}

	destPath, _, err := saveUpload(w, r, j.dir, "audio", "m4a")
	if err != nil {
		functions.JSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Minute)
	defer cancel()
	duration, err := ffprobeDuration(ctx, destPath)
	if err != nil {
		functions.JSONError(w, http.StatusBadRequest, "unreadable audio file: "+err.Error())
		return
	}

	j.mu.Lock()
	j.audioPath = destPath
	j.audioDuration = duration
	j.mu.Unlock()

	functions.WriteJSON(w, http.StatusOK, map[string]any{"duration": duration})
}

// serveSource handles GET /api/video-tool/source/{id}: the loaded/uploaded source, for the browser's <video>
// preview. http.ServeFile handles Range requests on its own, which is what makes the preview scrubbable.
func serveSource(w http.ResponseWriter, r *http.Request) {
	j, ok := getJob(mux.Vars(r)["id"])
	if !ok {
		functions.JSONError(w, http.StatusNotFound, "job not found or expired")
		return
	}
	j.mu.Lock()
	path := j.sourcePath
	j.mu.Unlock()
	if path == "" {
		functions.JSONError(w, http.StatusNotFound, "source not loaded yet")
		return
	}
	http.ServeFile(w, r, path)
}

// exportJob handles GET /api/video-tool/export/{id}/events?format=&speed=&pitch=&volume=&start=&end=&useAudioTrack=
// &audioMode=&audioStart=&audioPitch=&audioVolume=: runs ffmpeg against the job's already-loaded source (+ its audio
// track, if attached and requested), streaming progress as SSE, ending with "complete" {id, filename} — the same id
// then downloads via GET /api/video-tool/download/{id} — or "error".
func exportJob(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	j, jobOK := getJob(id)

	send, ok := startSSE(w, r)
	if !ok {
		return
	}
	if !jobOK {
		send("error", map[string]string{"message": "job not found or expired"})
		return
	}

	j.mu.Lock()
	srcPath := j.sourcePath
	sourceDuration := j.sourceDuration
	audioPath := j.audioPath
	j.mu.Unlock()
	if srcPath == "" {
		send("error", map[string]string{"message": "source not loaded yet"})
		return
	}

	format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	if !allowedFormats[format] {
		send("error", map[string]string{"message": "unsupported format: " + format})
		return
	}

	trackPath := ""
	if r.URL.Query().Get("useAudioTrack") == "1" && audioPath != "" {
		trackPath = audioPath
	}
	audioMode := r.URL.Query().Get("audioMode")
	if audioMode != "replace" {
		audioMode = "mix"
	}

	params := editParams{
		Speed:  clamp(floatParam(r, "speed", 1), 0.25, 4),
		Pitch:  clamp(floatParam(r, "pitch", 0), -12, 12),
		Volume: clamp(floatParam(r, "volume", 1), 0, 5),
		Start:  clamp(floatParam(r, "start", 0), 0, sourceDuration),
		End:    clamp(floatParam(r, "end", sourceDuration), 0, sourceDuration),
		Format: format,

		AudioTrackPath: trackPath,
		AudioMode:      audioMode,
		AudioStart:     floatParam(r, "audioStart", 0),
		AudioSpeed:     clamp(floatParam(r, "audioSpeed", 1), 0.25, 4),
		AudioPitch:     clamp(floatParam(r, "audioPitch", 0), -12, 12),
		AudioVolume:    clamp(floatParam(r, "audioVolume", 1), 0, 5),
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Minute)
	defer cancel()

	effectiveDuration := params.End - params.Start
	if effectiveDuration <= 0 {
		effectiveDuration = sourceDuration
	}
	outputDuration := effectiveDuration
	if params.Speed > 0 {
		outputDuration = effectiveDuration / params.Speed
	}

	send("status", map[string]string{"message": "Processing…"})
	outPath, err := runFFmpeg(ctx, srcPath, params, j.dir, sourceDuration, outputDuration, func(pct float64) {
		send("edit_progress", map[string]float64{"percent": pct})
	})
	if err != nil {
		send("error", map[string]string{"message": "processing failed: " + err.Error()})
		return
	}

	j.mu.Lock()
	name := sanitizeFilename(j.title)
	if name == "" {
		name = "video"
	}
	filename := name + "." + format
	j.outputPath = outPath
	j.outputFilename = filename
	j.outputMime = formatMime(format)
	j.mu.Unlock()

	log.Infof("video-tool: exported %s for job %s", filename, id)
	send("complete", map[string]string{"id": id, "filename": filename})
}

// downloadResult handles GET /api/video-tool/download/{id}: the job's exported output, once exportJob has produced
// one. The whole job (source, audio track, output) is swept together after retention — see jobs.go.
func downloadResult(w http.ResponseWriter, r *http.Request) {
	j, ok := getJob(mux.Vars(r)["id"])
	if !ok {
		functions.JSONError(w, http.StatusNotFound, "result not found or expired")
		return
	}
	j.mu.Lock()
	path, filename, mimeType := j.outputPath, j.outputFilename, j.outputMime
	j.mu.Unlock()
	if path == "" {
		functions.JSONError(w, http.StatusNotFound, "not exported yet")
		return
	}
	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	http.ServeFile(w, r, path)
}
