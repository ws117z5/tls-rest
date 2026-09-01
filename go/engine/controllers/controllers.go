package controllers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"text/template"

	config "tls-rest/go/constants"

	"tls-rest/go/engine/controllers/auth"
	"tls-rest/go/engine/controllers/db/pgdb"
	"tls-rest/go/engine/controllers/module"

	"tls-rest/go/engine/controllers/db/cache"
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
	session, ok := r.Context().Value(auth.SESSION_KEY).(*cache.Session)
	if ok && session != nil && session.UserID > 0 {
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
	managed := make([]string, 0, len(auth.ModuleDefaults()))
	available := make([]string, 0, len(auth.ModuleDefaults()))
	for module := range auth.ModuleDefaults() {
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

// ModulesAPI handles GET /api/modules. It returns every module registered in Go
// that the current user has at least one mode on, each with its name/description/
// endpoint and the modes the user may perform. The list is driven by the engine
// registry (module.RegisteredModuleMenu) — no go.config.json — so a newly
// registered module appears automatically. The frontend uses this to build the
// menu and gate per mode.
//
// Modes are returned as names (e.g. ["list","view","edit"]) rather than a raw
// bitmask so the client never depends on this package's bit layout.
// ModulesAPI handles GET /api/modules and returns the complete menu the current
// user may see, already privilege-filtered server-side:
//
//	{ "head": [ ...top-level modules/pages... ],
//	  "submenus": { "engine": [...], "pages": [...] } }
//
// Each module/page declares an optional Submenu (a submenu title); entries with
// no submenu go in "head", the rest are grouped under submenus[title]. Because
// the list is already access-filtered, per-entry requiresAuth/requiresAdmin and a
// top-level isAdmin are intentionally NOT returned — the client renders whatever
// it's given.
func ModulesAPI(w http.ResponseWriter, r *http.Request) {
	userID := 0
	isAdmin := false
	var rights auth.ModuleModeRights

	if s, ok := r.Context().Value(auth.SESSION_KEY).(*cache.Session); ok && s != nil {
		userID = s.UserID
		isAdmin = s.IsAdmin
		rights = s.ModuleModes
	}
	if rights == nil {
		rights = auth.ResolveModuleModeRights(userID)
	}

	// head = top-level entries; submenus groups entries by submenu title. Entries
	// are heterogeneous (modules vs pages), hence interface{}.
	head := make([]interface{}, 0)
	submenus := map[string][]interface{}{}
	add := func(submenu string, entry interface{}) {
		if submenu == "" {
			head = append(head, entry)
		} else {
			submenus[submenu] = append(submenus[submenu], entry)
		}
	}

	// --- Modules: authoritative list from the Go registry (auth.ModuleDefaults),
	// enriched with description/submenu from the menu registry when available. ---
	type moduleEntry struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Endpoint    string   `json:"endpoint"`
		Modes       []string `json:"modes"`
		Icon        string   `json:"icon,omitempty"`
	}
	menuByID := map[string]module.ModuleMenuMeta{}
	for _, m := range module.RegisteredModuleMenu() {
		menuByID[m.ID] = m
	}
	ids := make([]string, 0, len(auth.ModuleDefaults()))
	for id := range auth.ModuleDefaults() {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if m, ok := module.RegisteredModules[id]; ok && m.IsHidden() {
			continue // API-only module: no menu / no web route
		}
		mask := auth.AllowedModes(rights, id, isAdmin)
		if mask == 0 {
			continue // no access
		}
		desc := id
		submenu := ""
		icon := ""
		readOnly := false
		if m, ok := module.RegisteredModules[id]; ok {
			readOnly = m.IsReadOnly() // reliable, independent of the menu writer
		}
		if meta, ok := menuByID[id]; ok {
			if meta.Description != "" {
				desc = meta.Description
			} else if meta.Name != "" {
				desc = meta.Name
			}
			submenu = meta.Submenu
			icon = meta.Icon
		}
		modeNames := auth.ModeNames(mask)
		if readOnly {
			modeNames = intersectModes(modeNames, []string{"list", "view"})
		}
		if m, ok := module.RegisteredModules[id]; ok {
			if hidden := m.GetHiddenModes(); len(hidden) > 0 {
				modeNames = removeModes(modeNames, hidden)
			}
		}
		add(submenu, moduleEntry{
			Name:        id,
			Description: desc,
			Endpoint:    "/" + id,
			Modes:       modeNames,
			Icon:        icon,
		})
	}

	// --- Pages: from the page registry, session-filtered. No requiresAuth/Admin
	// in the output (already filtered); an endpoint is included. ---
	type pageEntry struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Endpoint string `json:"endpoint"`
		Icon     string `json:"icon,omitempty"`
	}
	for _, p := range module.RegisteredPageMenu() {
		if p.RequiresAdmin && !isAdmin {
			continue
		}
		if p.RequiresAuth && userID == 0 {
			continue
		}
		add(p.Submenu, pageEntry{
			ID:       p.ID,
			Name:     p.Name,
			Endpoint: "/" + p.ID,
			Icon:     p.Icon,
		})
	}

	// user identity for the menu (login/logout swap + avatar). Kept separate from
	// module access (already filtered): the client uses `authenticated` to swap
	// the login item for a logout button and shows `avatar` when present.
	user := map[string]interface{}{
		"authenticated": userID > 0,
	}
	if userID > 0 {
		if av := userAvatar(userID); av != "" {
			user["avatar"] = av
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"user":     user,
		"head":     head,
		"submenus": submenus,
	})
}

// removeModes returns `have` without any mode present in `drop`.
func removeModes(have, drop []string) []string {
	dropped := map[string]bool{}
	for _, d := range drop {
		dropped[d] = true
	}
	out := make([]string, 0, len(have))
	for _, m := range have {
		if !dropped[m] {
			out = append(out, m)
		}
	}
	return out
}

// intersectModes returns the mode names present in both lists, preserving the
// order of `have`. Used to cap a read-only module's advertised modes.
func intersectModes(have, allow []string) []string {
	allowed := map[string]bool{}
	for _, a := range allow {
		allowed[a] = true
	}
	out := make([]string, 0, len(have))
	for _, m := range have {
		if allowed[m] {
			out = append(out, m)
		}
	}
	return out
}

// userAvatar returns the menu avatar URL for a user, or "" if none. users.image
// is a type_image field: a JSON array of refs (each already carrying a
// "/image/<uuid>.<ext>" url). Use the first ref's url.
func userAvatar(userID int) string {
	db, err := pgdb.GetInstance()
	if err != nil {
		return ""
	}
	row, err := db.GetOne(`SELECT image FROM users WHERE id = $1`, userID)
	if err != nil || row == nil {
		return ""
	}
	raw, _ := row["image"].(string)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	var refs []struct {
		URL  string `json:"url"`
		UUID string `json:"uuid"`
	}
	if err := json.Unmarshal([]byte(raw), &refs); err == nil && len(refs) > 0 {
		if refs[0].URL != "" {
			return refs[0].URL
		}
		if refs[0].UUID != "" {
			return "/image/" + refs[0].UUID
		}
		return ""
	}

	// Back-compat: a value stored as a plain url or uuid string.
	if strings.HasPrefix(raw, "/image/") {
		return raw
	}
	return "/image/" + raw
}

// PagesAPI handles GET /api/pages. It returns the backend-registered pages
// (module.RegisteredPageMenu) the current session may access — admin-only pages
// are omitted for non-admins, auth-only pages for anonymous visitors. Like
// /api/modules it is self-filtering. The frontend matches each id to its page
// component (by href) and builds the pages menu, so a newly registered page
// appears automatically without editing the client.
func PagesAPI(w http.ResponseWriter, r *http.Request) {
	userID := 0
	isAdmin := false
	if s, ok := r.Context().Value(auth.SESSION_KEY).(*cache.Session); ok && s != nil {
		userID = s.UserID
		isAdmin = s.IsAdmin
	}

	type pageInfo struct {
		ID            string `json:"id"`
		Name          string `json:"name"`
		RequiresAuth  bool   `json:"requiresAuth"`
		RequiresAdmin bool   `json:"requiresAdmin"`
	}

	metas := module.RegisteredPageMenu()
	out := make([]pageInfo, 0, len(metas))
	for _, p := range metas {
		if p.RequiresAdmin && !isAdmin {
			continue
		}
		if p.RequiresAuth && userID == 0 {
			continue
		}
		out = append(out, pageInfo{
			ID: p.ID, Name: p.Name,
			RequiresAuth: p.RequiresAuth, RequiresAdmin: p.RequiresAdmin,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"isAdmin": isAdmin,
		"pages":   out,
	})
}
