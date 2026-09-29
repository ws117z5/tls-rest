// The model server (see the API doc in js/src/pages/transformerstudio) runs on whatever Mac the user points the web UI
// at, so the browser can't call it directly: it may be on the LAN, unreachable from outside, and its CORS/TLS story is
// out of scope. This file has the Go server fetch it instead — the browser's "server" field never leaves this process.
//
// Every route here requires the page's own rights grant (RequiresAuth + PageAbstract.guard's MODE_EDIT — see
// transformerstudio.go), so this is not a public relay. Even so, "server" is user-supplied, so proxyRequest only
// forwards to a closed allowlist of documented API paths, not an arbitrary path on that host.
package transformerstudio

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"tls-rest/go/engine/controllers/functions"
)

const (
	maxProxyBody = 32 << 20  // JSON requests/responses, including a generated image
	maxVideoBody = 512 << 20 // an uploaded or generated video
)

// allowedCalls is the closed set of remote endpoints proxyRequest may reach — the model server's own documented API,
// minus /events (SSE, its own route) and /enhance/video (multipart, its own route).
var allowedCalls = map[string]bool{
	"GET /api/v1/status":           true,
	"GET /api/v1/models":           true,
	"POST /api/v1/models":          true,
	"POST /api/v1/models/select":   true,
	"POST /api/v1/config":          true,
	"POST /api/v1/tokenizers":      true,
	"POST /api/v1/train/start":     true,
	"POST /api/v1/train/stop":      true,
	"POST /api/v1/translate":       true,
	"POST /api/v1/images/generate": true,
	"POST /api/v1/enhance":         true,
	"GET /api/v1/expression":       true,
	"POST /api/v1/expression":      true,
	"GET /api/v1/video":            true,
	"POST /api/v1/video":           true,
}

// passthroughHeaders are the model server's own response headers worth surfacing (generation timings, seeds, ...).
var passthroughHeaders = []string{
	"X-Seed", "X-Generation-Ms", "X-Enhance-Ms", "X-Frames", "X-Face-Found", "X-Edit-Ms",
}

// remoteBase validates a user-supplied "server" value and returns its normalized origin (scheme://host[:port], no path).
func remoteBase(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("server is required")
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", errors.New("invalid server address")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", errors.New("server must be http or https")
	}
	return u.Scheme + "://" + u.Host, nil
}

type proxyEnvelope struct {
	Server string          `json:"server"`
	Method string          `json:"method"`
	Path   string          `json:"path"`
	Body   json.RawMessage `json:"body,omitempty"`
}

// proxyRequest handles POST /api/transformer-studio/proxy {server, method, path, body?}: forwards one call to the
// model server's JSON API (a path from allowedCalls only) and mirrors back its status, content type and body —
// verbatim, whether that's a JSON object or a generated PNG.
func proxyRequest(w http.ResponseWriter, r *http.Request) {
	var in proxyEnvelope
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&in); err != nil {
		functions.JSONError(w, http.StatusBadRequest, "invalid request")
		return
	}
	base, err := remoteBase(in.Server)
	if err != nil {
		functions.JSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	method := strings.ToUpper(strings.TrimSpace(in.Method))
	if method == "" {
		method = http.MethodGet
	}
	if !allowedCalls[method+" "+in.Path] {
		functions.JSONError(w, http.StatusBadRequest, "unsupported call: "+method+" "+in.Path)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	var body io.Reader
	if len(in.Body) > 0 {
		body = strings.NewReader(string(in.Body))
	}
	req, err := http.NewRequestWithContext(ctx, method, base+in.Path, body)
	if err != nil {
		functions.JSONError(w, http.StatusBadRequest, "invalid path")
		return
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		functions.JSONError(w, http.StatusBadGateway, "model server unreachable: "+err.Error())
		return
	}
	defer resp.Body.Close()

	copyProxyResponse(w, resp, maxProxyBody)
}

// proxyEnhanceVideo handles POST /api/transformer-studio/enhance-video?server=...: streams the incoming multipart
// upload straight through to the model server's own /enhance/video and mirrors its response (an mp4).
func proxyEnhanceVideo(w http.ResponseWriter, r *http.Request) {
	base, err := remoteBase(r.URL.Query().Get("server"))
	if err != nil {
		functions.JSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		functions.JSONError(w, http.StatusBadRequest, "expected a multipart video upload")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/v1/enhance/video",
		http.MaxBytesReader(w, r.Body, maxVideoBody))
	if err != nil {
		functions.JSONError(w, http.StatusBadRequest, "invalid server address")
		return
	}
	req.Header.Set("Content-Type", r.Header.Get("Content-Type"))
	req.ContentLength = r.ContentLength

	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		functions.JSONError(w, http.StatusBadGateway, "model server unreachable: "+err.Error())
		return
	}
	defer resp.Body.Close()

	copyProxyResponse(w, resp, maxVideoBody)
}

// copyProxyResponse mirrors a model-server response onto w: status, content type, the timing/seed headers it sets, and
// the body (capped at max bytes so a runaway remote can't exhaust this process's memory).
func copyProxyResponse(w http.ResponseWriter, resp *http.Response, max int64) {
	for _, h := range passthroughHeaders {
		if v := resp.Header.Get(h); v != "" {
			w.Header().Set(h, v)
		}
	}
	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = "application/json"
	}
	w.Header().Set("Content-Type", ct)
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, io.LimitReader(resp.Body, max))
}

// proxyEvents handles GET /api/transformer-studio/events?server=...: relays the model server's SSE stream to the
// browser so it never has to open a direct cross-origin connection to it.
func proxyEvents(w http.ResponseWriter, r *http.Request) {
	base, err := remoteBase(r.URL.Query().Get("server"))
	if err != nil {
		functions.JSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, base+"/api/v1/events", nil)
	if err != nil {
		functions.JSONError(w, http.StatusBadRequest, "invalid server address")
		return
	}
	req.Header.Set("Accept", "text/event-stream")

	// No client timeout: this stream is meant to stay open; it ends when the browser disconnects (r.Context()) or the
	// model server closes it.
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		functions.JSONError(w, http.StatusBadGateway, "model server unreachable: "+err.Error())
		return
	}
	defer resp.Body.Close()

	flusher, ok := w.(http.Flusher)
	if !ok {
		functions.JSONError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				return
			}
			flusher.Flush()
		}
		if err != nil {
			return
		}
	}
}
