// Package request provides a single, request-scoped view of every parameter
// source for one HTTP request: the URL query, the parsed body (JSON object,
// x-www-form-urlencoded, or multipart/form-data), the route's path variables,
// and values a handler injects with Set.
//
// The parsed result is cached on the *Request, which is shared through the
// request context, so every layer — middleware, controller, the fieldset
// engine, data hooks — reads the same values, including anything injected
// mid-request. The raw body is read exactly once and restored on r.Body, so
// code that still reads r.Body directly keeps working.
package request

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/gorilla/mux"
)

type ctxKey struct{}

const maxMemory = 32 << 20 // 32 MiB, for multipart parsing

// Request is the unified parameter bag. Lookup precedence, highest first:
// Set > path variable > body > query.
type Request struct {
	r *http.Request

	once     sync.Once
	query    map[string]any // collapsed URL query
	body     map[string]any // parsed body (object-shaped bodies only)
	raw      []byte         // raw body bytes, any content type
	bodyErr  error          // body decode error, if any
	mimeType string         // parsed media type of the request body

	mu        sync.Mutex
	overrides map[string]any // Set()
}

// New builds a Request from r, reading and restoring the body immediately. Call
// it once per request (the middleware does); handlers use From.
func New(r *http.Request) *Request {
	rq := &Request{r: r}
	rq.parse()
	return rq
}

// ContextWith returns ctx carrying rq (for middleware that builds a context
// incrementally). Handlers usually use WithRequest.
func ContextWith(ctx context.Context, rq *Request) context.Context {
	return context.WithValue(ctx, ctxKey{}, rq)
}

// WithRequest returns a shallow copy of r whose context carries rq.
func WithRequest(r *http.Request, rq *Request) *http.Request {
	return r.WithContext(ContextWith(r.Context(), rq))
}

// From returns the Request bag installed for r, or a detached one built from
// whatever of r is still readable. The installed instance is shared, so Set on
// it is visible to every later reader.
func From(r *http.Request) *Request {
	if rq, ok := r.Context().Value(ctxKey{}).(*Request); ok {
		rq.r = r // keep mux.Vars current for this call site
		return rq
	}
	return New(r)
}

func (rq *Request) parse() {
	rq.once.Do(func() {
		rq.query = collapse(rq.r.URL.Query())
		rq.body = map[string]any{}
		rq.mimeType, _, _ = mime.ParseMediaType(rq.r.Header.Get("Content-Type"))

		if rq.r.Body == nil {
			return
		}
		raw, err := io.ReadAll(rq.r.Body)
		if err != nil {
			rq.bodyErr = err
			return
		}
		rq.raw = raw
		rq.r.Body = io.NopCloser(bytes.NewReader(raw)) // restore for direct readers
		if len(raw) == 0 {
			return
		}

		switch rq.mimeType {
		case "application/x-www-form-urlencoded":
			vals, err := url.ParseQuery(string(raw))
			if err != nil {
				rq.bodyErr = err
				return
			}
			rq.body = collapse(vals)
		case "multipart/form-data":
			if err := rq.r.ParseMultipartForm(maxMemory); err != nil {
				rq.bodyErr = err
			} else if rq.r.MultipartForm != nil {
				rq.body = collapse(rq.r.MultipartForm.Value)
			}
			rq.r.Body = io.NopCloser(bytes.NewReader(raw)) // ParseMultipartForm drained it
		default:
			// application/json, text/json, or unset (many clients omit it): treat
			// an object as fields; an array or scalar is available via Raw/Bind.
			var obj map[string]any
			if err := json.Unmarshal(raw, &obj); err != nil {
				rq.bodyErr = err
			} else {
				rq.body = obj
			}
		}
	})
}

// --- reads ---------------------------------------------------------------

// Get resolves name across all sources by precedence.
func (rq *Request) Get(name string) (any, bool) {
	rq.mu.Lock()
	if v, ok := rq.overrides[name]; ok {
		rq.mu.Unlock()
		return v, true
	}
	rq.mu.Unlock()

	if v, ok := mux.Vars(rq.r)[name]; ok {
		return v, true
	}
	if v, ok := rq.body[name]; ok {
		return v, true
	}
	if v, ok := rq.query[name]; ok {
		return v, true
	}
	return nil, false
}

// Has reports whether name is present in any source.
func (rq *Request) Has(name string) bool {
	_, ok := rq.Get(name)
	return ok
}

// String returns name as a string ("" when absent). A multi-value param yields
// its first value.
func (rq *Request) String(name string) string {
	v, ok := rq.Get(name)
	if !ok {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case []string:
		if len(t) > 0 {
			return t[0]
		}
		return ""
	case fmt.Stringer:
		return t.String()
	default:
		return fmt.Sprintf("%v", t)
	}
}

// Int returns name as an int (0 when absent or unparseable). Handles JSON
// numbers (float64) and numeric strings.
func (rq *Request) Int(name string) int {
	switch t := mustGet(rq, name).(type) {
	case int:
		return t
	case int64:
		return int(t)
	case float64:
		return int(t)
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(t))
		return n
	}
	return 0
}

// Bool returns name as a bool (false when absent or unparseable).
func (rq *Request) Bool(name string) bool {
	switch t := mustGet(rq, name).(type) {
	case bool:
		return t
	case string:
		b, _ := strconv.ParseBool(strings.TrimSpace(t))
		return b
	}
	return false
}

// Body returns a copy of the parsed request body only (no query, path vars, or
// overrides) — the map to persist as a record on create/update.
func (rq *Request) Body() map[string]any {
	out := make(map[string]any, len(rq.body))
	for k, v := range rq.body {
		out[k] = v
	}
	return out
}

// BodyErr returns the body decode error, if the body was present but malformed
// for its declared content type.
func (rq *Request) BodyErr() error { return rq.bodyErr }

// All returns every value merged by precedence (query < body < path var < Set).
func (rq *Request) All() map[string]any {
	out := make(map[string]any, len(rq.query)+len(rq.body)+8)
	for k, v := range rq.query {
		out[k] = v
	}
	for k, v := range rq.body {
		out[k] = v
	}
	for k, v := range mux.Vars(rq.r) {
		out[k] = v
	}
	rq.mu.Lock()
	for k, v := range rq.overrides {
		out[k] = v
	}
	rq.mu.Unlock()
	return out
}

// Raw returns the raw request body bytes (any content type).
func (rq *Request) Raw() []byte { return rq.raw }

// Bind JSON-unmarshals the raw body into v (for typed or array-shaped bodies).
func (rq *Request) Bind(v any) error { return json.Unmarshal(rq.raw, v) }

// --- writes ---------------------------------------------------------------

// Set injects a value that overrides every other source for name, visible to
// every later reader of the shared bag.
func (rq *Request) Set(name string, value any) {
	rq.mu.Lock()
	if rq.overrides == nil {
		rq.overrides = map[string]any{}
	}
	rq.overrides[name] = value
	rq.mu.Unlock()
}

// --- helpers ------------------------------------------------------------

func mustGet(rq *Request, name string) any {
	v, _ := rq.Get(name)
	return v
}

// collapse turns a url.Values-shaped map into name -> value, keeping a []string
// only when a key genuinely repeats.
func collapse(m map[string][]string) map[string]any {
	out := make(map[string]any, len(m))
	for k, vs := range m {
		switch len(vs) {
		case 0:
			out[k] = ""
		case 1:
			out[k] = vs[0]
		default:
			cp := make([]string, len(vs))
			copy(cp, vs)
			out[k] = cp
		}
	}
	return out
}
