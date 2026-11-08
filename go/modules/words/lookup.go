package words

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"tls-rest/go/engine/controllers/db/cache"
	"tls-rest/go/engine/controllers/functions"
)

// Forces HTTP/1.1: verified live (go/engine, 4 runs each way) that Google's
// abuse detection on this free, unofficial endpoint 429s Go's HTTP/2 client
// every single time while its HTTP/1.1 client succeeds every single time —
// TLS curve preferences made no difference, only the HTTP version did.
// TLSNextProto set to an empty map is the standard way to opt a Transport
// out of the automatic HTTP/2 upgrade net/http otherwise performs.
var translateClient = &http.Client{
	Timeout:   10 * time.Second,
	Transport: &http.Transport{TLSNextProto: map[string]func(string, *tls.Conn) http.RoundTripper{}},
}

// lookupHandler handles GET /words/lookup?word=<word>&to=<lang>: detects the
// word's language and fetches translations in one request, via the same
// free, unofficial Google Translate endpoint words-android's own
// GoogleTranslator uses (sl=auto also returns the detected source language,
// see lookupTranslation) — kept consistent with that client rather than
// adding a second, different translation source for the two apps to drift
// against.
//
// Route: GET /words/lookup  (registered in NewWords).
func (m *Words) lookupHandler(w http.ResponseWriter, r *http.Request) {
	if s := cache.SessionFromContext(r.Context()); s == nil || s.UserID <= 0 {
		functions.JSONError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	word := strings.TrimSpace(r.URL.Query().Get("word"))
	if word == "" {
		functions.JSONError(w, http.StatusBadRequest, "word is required")
		return
	}
	to := r.URL.Query().Get("to")
	if to == "" {
		to = "ru"
	}

	language, translations, err := lookupTranslation(word, to)
	if err != nil {
		log.Printf("[words] lookup(%q -> %q) failed: %v", word, to, err)
		functions.JSONError(w, http.StatusBadGateway, "translation lookup failed")
		return
	}

	functions.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"language":     language,
		"translations": translations,
	})
}

// lookupTranslation calls translate.googleapis.com's single endpoint with
// sl=auto, dt=t (main translation) + dt=bd (bilingual dictionary
// alternatives) — the exact same response shape words-android's
// GoogleTranslator.parse() reads, plus the detected source language, which
// that endpoint returns at index 2 only when sl=auto was requested.
func lookupTranslation(word, to string) (language string, translations []string, err error) {
	q := url.Values{
		"client": {"gtx"},
		"sl":     {"auto"},
		"tl":     {to},
		"dt":     {"t", "bd"},
		"q":      {word},
	}
	reqURL := "https://translate.googleapis.com/translate_a/single?" + q.Encode()

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := translateClient.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("translate http %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, err
	}

	var root []interface{}
	if err := json.Unmarshal(body, &root); err != nil {
		return "", nil, err
	}

	seen := map[string]bool{}
	add := func(text string) {
		text = strings.TrimSpace(text)
		if text != "" && !seen[text] {
			seen[text] = true
			translations = append(translations, text)
		}
	}

	// index 0: translated sentence chunks — each chunk's [0] is one translated piece.
	if len(root) > 0 {
		if sentences, ok := root[0].([]interface{}); ok {
			for _, chunk := range sentences {
				if arr, ok := chunk.([]interface{}); ok && len(arr) > 0 {
					if text, ok := arr[0].(string); ok {
						add(text)
					}
				}
			}
		}
	}
	// index 1: dictionary entries per part of speech — entry[1] is its alternatives.
	if len(root) > 1 {
		if entries, ok := root[1].([]interface{}); ok {
			for _, entry := range entries {
				arr, ok := entry.([]interface{})
				if !ok || len(arr) < 2 {
					continue
				}
				if terms, ok := arr[1].([]interface{}); ok {
					for _, term := range terms {
						if text, ok := term.(string); ok {
							add(text)
						}
					}
				}
			}
		}
	}
	// index 2: detected source language — only meaningful because sl=auto was requested.
	if len(root) > 2 {
		if lang, ok := root[2].(string); ok {
			language = lang
		}
	}
	if language == "" {
		language = "und"
	}

	return language, translations, nil
}
