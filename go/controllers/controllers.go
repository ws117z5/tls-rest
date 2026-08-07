package controllers

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"text/template"

	config "tls-rest/go/constants"

	"tls-rest/go/lib/auth"

	"tls-rest/go/lib/db/cache"
)

func Error(w http.ResponseWriter, r *http.Request, ErrID int) {
	session, ok := r.Context().Value(auth.SESSION_KEY).(*cache.Session)
	if !ok || session != nil {
		// Handle the case where session is not found or not valid
		log.Printf("Error: Session not found or invalid for error %d", ErrID)
		// Use session.ID, session.UserID, etc.
	}

	tpl := config.Tmpl{
		Title: "Error",
		Body: map[string]interface{}{
			"ErrID":   ErrID,
			"Message": http.StatusText(ErrID),
		},
	}

	if t, err := template.ParseFiles("templates/error.html"); err == nil {
		if err := t.Execute(w, tpl); err != nil {
			log.Println(err.Error())
			http.Error(w, http.StatusText(ErrID), ErrID)
		}
	}

	fmt.Fprintln(w, "http2 not supported!")
}

// sendEarlyHints emits a 103 Early Hints informational response advertising the
// critical assets (scripts, styles, images) via Link: rel=preload headers, so
// the browser can start fetching them before the full page is rendered.
func sendEarlyHints(w http.ResponseWriter) {
	h := w.Header()

	// Copy-append so we never mutate config.JsHeader's backing array.
	scripts := append(append([]string{}, config.JsHeader...), config.JsFooter...)
	for _, f := range scripts {
		h.Add("Link", "<"+f+">; rel=preload; as=script")
	}
	for _, f := range config.Css {
		h.Add("Link", "<"+f+">; rel=preload; as=style")
	}
	// NB: images are intentionally not preloaded here. The SPA shell is served
	// for every route, so preloading page-specific images (e.g. the index
	// background) globally makes the browser warn that they were "preloaded but
	// not used" on pages that don't reference them. Scripts and styles are always
	// needed, images are not.

	w.WriteHeader(http.StatusEarlyHints)
}

// Index page logic
func Index(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Determine authentication state from the session set by the middleware, so
	// the SPA can gate auth-only UI (e.g. the Users menu item).
	authenticated := false
	if session, ok := r.Context().Value(auth.SESSION_KEY).(*cache.Session); ok && session != nil && session.UserID > 0 {
		authenticated = true
	}

	// Send 103 Early Hints so the browser can begin fetching critical assets
	// while this handler renders the page. This is the modern replacement for
	// the deprecated (and browser-removed) HTTP/2 Server Push.
	sendEarlyHints(w)

	tpl := template.New("index.gohtml")

	tplData := config.Tmpl{
		JsHeader:  config.JsHeader,
		JsFooter:  config.JsFooter,
		CssHeader: config.Css,
		Img:       config.Img,
		Title:     "HelloWorld", //todo route.getTitle(r.URL.Path),
		Body: map[string]interface{}{
			"GoogleID":      config.GoogleID,
			"Authenticated": authenticated,
		},
	}

	//Function to mark preload images (somehow still not working on chrome )
	tpl.Funcs(template.FuncMap{
		"getImgType": func(s string) string {
			ext := s[strings.Index(s, ".")+1:]
			switch ext {
			case "png":
				return "image/png"
			case "jpeg":
			case "jpg":
				return "image/jpeg"
			case "svg":
				return "image/image/svg+xml"
			case "gif":
				return "image/gif"
			default:
				return "image/error"
			}

			return ""
		},
	})

	if t, err := tpl.ParseFiles("templates/index.gohtml"); err == nil {
		if err := t.Execute(w, tplData); err != nil {
			log.Println(err.Error())
			//http.Error(w, http.StatusText(500), 500)
		}
	} else {
		log.Println(err.Error())
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)

	}
}
