// Package videotool implements the Video Tool page: paste a youtube/instagram/tiktok (or any other yt-dlp-supported)
// link, pick an output format and edits (speed/pitch/volume/cut), and download the result. Processing runs
// synchronously within a single SSE request (see handlers.go streamProcess) — the same shape as the Network Mapper
// page (go/pages/netmapper) — ending with a "complete" event that carries a short-lived download id (see jobs.go).
package videotool

import (
	"net/http"
	"regexp"
	"strings"
	"time"

	"tls-rest/go/app"
	"tls-rest/go/engine/controllers/module"
)

const (
	maxDurationSeconds = 2 * 60 * 60 // reject sources longer than this (2 hours)
	maxFilesize        = "2G"        // yt-dlp --max-filesize
	maxFilesizeBytes   = 2 << 30     // same cap, enforced on direct uploads (2GiB)
	retention          = time.Hour   // how long a job's files stay on disk before being swept
	workDirPrefix      = "videotool-"
)

// allowedFormats is the closed set of output containers/codecs this page will produce.
var allowedFormats = map[string]bool{
	"mp4": true, "webm": true, "mkv": true,
	"mp3": true, "m4a": true, "wav": true, "opus": true,
}

// audioOnlyFormats drives whether ffmpeg strips video (-vn) for a given output format.
var audioOnlyFormats = map[string]bool{"mp3": true, "m4a": true, "wav": true, "opus": true}

var unsafeFilenameChars = regexp.MustCompile(`[^A-Za-z0-9 ._-]`)

// sanitizeFilename produces a safe Content-Disposition filename from the source's own (attacker-influenced) title.
func sanitizeFilename(s string) string {
	s = unsafeFilenameChars.ReplaceAllString(s, "_")
	s = strings.TrimSpace(s)
	if len(s) > 80 {
		s = s[:80]
	}
	return s
}

var unsafeExtChars = regexp.MustCompile(`[^a-zA-Z0-9]`)

// sanitizeExt extracts a short, safe filesystem extension (no dot) from an attacker-influenced upload filename,
// falling back to fallback when absent, unsafe, or implausibly long.
func sanitizeExt(ext, fallback string) string {
	ext = strings.TrimPrefix(ext, ".")
	ext = unsafeExtChars.ReplaceAllString(ext, "")
	if ext == "" || len(ext) > 5 {
		return fallback
	}
	return strings.ToLower(ext)
}

var Page = &module.PageAbstract{
	ID:           "video-tool",
	Name:         "Video Tool",
	Icon:         "tools",
	Submenu:      "tools",
	RequiresAuth: true,
	Routes: []module.PageRoute{
		{Path: "/api/video-tool/load", Methods: []string{http.MethodGet}, Handler: loadFromURL},
		{Path: "/api/video-tool/upload", Methods: []string{http.MethodPost}, Handler: uploadVideo},
		{Path: "/api/video-tool/upload-audio", Methods: []string{http.MethodPost}, Handler: uploadAudioTrack},
		{Path: "/api/video-tool/source/{id}", Methods: []string{http.MethodGet}, Handler: serveSource},
		{Path: "/api/video-tool/export/{id}/events", Methods: []string{http.MethodGet}, Handler: exportJob},
		{Path: "/api/video-tool/download/{id}", Methods: []string{http.MethodGet}, Handler: downloadResult},
	},
}

func init() { app.RegisterPage(Page) }
