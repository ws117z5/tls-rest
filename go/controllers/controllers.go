package controllers

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"text/template"

	config "github.com/ws117z5/tls-rest/go/constants"
	"github.com/ws117z5/tls-rest/go/lib/auth"
	"github.com/ws117z5/tls-rest/go/lib/db/cache"
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

// Index page logic
func Index(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Set-Cookie", "HttpOnly;Secure;SameSite=None")
	w.Header().Set("Content-type", "text/html")

	if pusher, ok := w.(http.Pusher); ok {
		push := func(path, ctype string) {
			if err := pusher.Push(path, &http.PushOptions{
				Method: "GET",
				Header: http.Header{
					"Accept-Encoding": r.Header["Accept-Encoding"],
					"Content-Type":    []string{ctype},
				},
			}); err != nil {
				log.Printf("push %s: %v", path, err)
			}
		}

		for _, file := range append(config.JsHeader, config.JsFooter...) {
			push(file, "application/javascript")
			push(file, "application/javascript")
		}

		for _, file := range config.Css {
			push(file, "text/css")
		}

		for _, file := range config.Img {
			push(file, "image/jpeg")
		}
	}

	tpl := template.New("index.gohtml")

	tplData := config.Tmpl{
		JsHeader:  config.JsHeader,
		JsFooter:  config.JsFooter,
		CssHeader: config.Css,
		Img:       config.Img,
		Title:     "HelloWorld", //todo route.getTitle(r.URL.Path),
		Body: map[string]interface{}{
			"GoogleID": config.GoogleID,
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
