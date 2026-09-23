package middleware

import (
	"bufio"
	"bytes"
	"net"
	"net/http"
)

// responseRecorder wraps an http.ResponseWriter to capture the final status code
// for access logging. It forwards Flush and Hijack so streaming responses (SSE
// for netmapper/opencv, WebSocket upgrades) keep working through the wrapper.
type responseRecorder struct {
	http.ResponseWriter
	status  int
	written bool
}

func newResponseRecorder(w http.ResponseWriter) *responseRecorder {
	return &responseRecorder{ResponseWriter: w, status: http.StatusOK}
}

func (rr *responseRecorder) WriteHeader(code int) {
	if !rr.written {
		rr.status = code
		rr.written = true
	}
	rr.ResponseWriter.WriteHeader(code)
}

func (rr *responseRecorder) Write(b []byte) (int, error) {
	rr.written = true
	return rr.ResponseWriter.Write(b)
}

func (rr *responseRecorder) Flush() {
	if f, ok := rr.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (rr *responseRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := rr.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

// Unwrap exposes the underlying ResponseWriter so http.ResponseController
// (SetWriteDeadline/SetReadDeadline) can reach through this wrapper — without
// it, a handler like GameEvents that needs to lift the server's blanket
// WriteTimeout for a long-lived SSE stream would silently fail to do so.
func (rr *responseRecorder) Unwrap() http.ResponseWriter {
	return rr.ResponseWriter
}

// statsBuffer holds a response entirely in memory instead of forwarding
// writes, so the middleware can add the X-Query-Stats-* headers *after* the
// handler finishes (query totals aren't known until then) and still have
// them land on the same response — no follow-up request, so nothing can ever
// race it. Only installed for requests opted into instrumentation (see
// middleware.go), never for streaming responses (SSE/WebSocket), which don't
// carry the opt-in header in the first place.
type statsBuffer struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func newStatsBuffer(w http.ResponseWriter) *statsBuffer {
	return &statsBuffer{ResponseWriter: w}
}

func (sb *statsBuffer) WriteHeader(code int) {
	if sb.status == 0 {
		sb.status = code
	}
}

func (sb *statsBuffer) Write(b []byte) (int, error) {
	return sb.body.Write(b)
}

// release writes the buffered status/body through to the real writer, after
// the caller has set any extra headers it wants on the response.
func (sb *statsBuffer) release() {
	if sb.status == 0 {
		sb.status = http.StatusOK
	}
	sb.ResponseWriter.WriteHeader(sb.status)
	sb.ResponseWriter.Write(sb.body.Bytes())
}
