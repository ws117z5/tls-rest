package controllers

import (
	"encoding/json"
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
	// the SPA can gate auth-only UI (e.g. the Users menu item) and admin-only
	// columns (id/uuid/created/updated/created_by in list/view).
	authenticated := false
	isAdmin := false
	userID := 0
	if session, ok := r.Context().Value(auth.SESSION_KEY).(*cache.Session); ok && session != nil && session.UserID > 0 {
		authenticated = true
		isAdmin = session.IsAdmin
		userID = session.UserID
	}

	// Modules the backend governs (managed), and the subset this user may access
	// (available). The SPA only loads modules that are available; a managed module
	// the user has no rights to is simply absent from "available", so the frontend
	// never loads it. Modules the backend doesn't govern (custom/frontend pages)
	// are not in "managed" and are always loaded.
	rights := auth.ResolveModuleModeRights(userID)
	managed := make([]string, 0, len(auth.ModuleDefaultPermissions))
	available := make([]string, 0, len(auth.ModuleDefaultPermissions))
	for module := range auth.ModuleDefaultPermissions {
		managed = append(managed, module)
		// Available (loadable / shown in the menu) when the user has any mode on
		// the module. Admins get every mode; anonymous users get module defaults.
		if auth.AllowedModes(rights, module, isAdmin) != 0 {
			available = append(available, module)
		}
	}
	managedJSON, _ := json.Marshal(managed)
	availableJSON, _ := json.Marshal(available)

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
			"GoogleID":             config.GoogleID,
			"Authenticated":        authenticated,
			"IsAdmin":              isAdmin,
			"ManagedModulesJSON":   string(managedJSON),
			"AvailableModulesJSON": string(availableJSON),
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

// ModulesAPI handles GET /api/modules. It returns the modules declared in
// go.config.json that (a) have a Go definition registered and (b) the current
// user has at least one mode on, each with its name/description/endpoint and the
// list of modes the user may perform. The frontend uses this to build the menu
// and gate access per mode; it then checks which modules also have a page
// defined client-side.
//
// Modes are returned as names (e.g. ["list","view","edit"]) rather than a raw
// bitmask so the client never depends on this package's bit layout.
func ModulesAPI(w http.ResponseWriter, r *http.Request) {
	userID := 0
	isAdmin := false
	var rights auth.ModuleModeRights

	if s, ok := r.Context().Value(auth.SESSION_KEY).(*cache.Session); ok && s != nil {
		userID = s.UserID
		isAdmin = s.IsAdmin
		rights = s.ModuleModes
	}
	// Defensive fallback if the session was created before rights were resolved.
	if rights == nil {
		rights = auth.ResolveModuleModeRights(userID)
	}

	type moduleInfo struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Endpoint    string   `json:"endpoint"`
		Modes       []string `json:"modes"`
	}

	out := make([]moduleInfo, 0, len(config.Config.Modules))
	for _, m := range config.Config.Modules {
		// Only expose modules that actually have a Go definition registered.
		if _, defined := auth.ModuleDefaultPermissions[m.Name]; !defined {
			continue
		}
		mask := auth.AllowedModes(rights, m.Name, isAdmin)
		if mask == 0 {
			continue // user has no access to this module
		}
		out = append(out, moduleInfo{
			Name:        m.Name,
			Description: m.Description,
			Endpoint:    m.Endpoint,
			Modes:       auth.ModeNames(mask),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"isAdmin": isAdmin,
		"modules": out,
	})
}
