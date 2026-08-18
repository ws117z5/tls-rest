package auth

import (
	"net/http"
	"strings"

	"tls-rest/go/engine/modules/users"

	"github.com/gorilla/mux"
)

// Auth is the OAuth entry point for every provider, wired at
// /users/Auth/{authType}. authType is "<Provider>Login" to start a flow or
// "<Provider>Callback" to complete it, e.g. GoogleLogin / GoogleCallback,
// GithubLogin / GithubCallback, FacebookLogin / FacebookCallback,
// VkLogin / VkCallback. Providers are declared once in providers.go.
func Auth(w http.ResponseWriter, r *http.Request) {
	authType, ok := mux.Vars(r)["authType"]
	if !ok {
		http.NotFound(w, r)
		return
	}

	name, action := parseAuthType(authType)
	p := providers[name]
	if p == nil || action == "" {
		http.NotFound(w, r)
		return
	}

	switch action {
	case "login":
		startOAuth(w, r, p)
	case "callback":
		completeOAuth(w, r, p)
	}
}

// parseAuthType splits "<Provider>Login"/"<Provider>Callback" into a canonical
// provider key (lower-case) and the action.
func parseAuthType(authType string) (name, action string) {
	switch {
	case strings.HasSuffix(authType, "Callback"):
		return strings.ToLower(strings.TrimSuffix(authType, "Callback")), "callback"
	case strings.HasSuffix(authType, "Login"):
		return strings.ToLower(strings.TrimSuffix(authType, "Login")), "login"
	}
	return "", ""
}

// startOAuth kicks off the provider flow: set a CSRF state cookie and redirect
// the user to the provider's consent screen.
func startOAuth(w http.ResponseWriter, r *http.Request, p *providerDef) {
	if !p.configured() {
		http.Redirect(w, r, "/login?error=provider_unconfigured", http.StatusTemporaryRedirect)
		return
	}
	state := setStateCookie(w, r)
	http.Redirect(w, r, p.config(r).AuthCodeURL(state), http.StatusTemporaryRedirect)
}

// completeOAuth handles the provider redirect back: verify state, exchange the
// code, fetch + normalize the account, then find/create the user and log in.
func completeOAuth(w http.ResponseWriter, r *http.Request, p *providerDef) {
	if !checkStateCookie(w, r) {
		http.Redirect(w, r, "/login?error=state", http.StatusTemporaryRedirect)
		return
	}
	code := r.FormValue("code")
	if code == "" {
		http.Redirect(w, r, "/login?error=oauth", http.StatusTemporaryRedirect)
		return
	}

	cfg := p.config(r)
	tok, err := cfg.Exchange(r.Context(), code)
	if err != nil {
		http.Redirect(w, r, "/login?error=oauth", http.StatusTemporaryRedirect)
		return
	}

	acc, err := p.fetch(r.Context(), cfg, tok)
	if err != nil {
		http.Redirect(w, r, "/login?error=oauth", http.StatusTemporaryRedirect)
		return
	}
	acc.Provider = p.name

	userID, username, err := users.FindOrCreateOAuthUser(acc)
	if err != nil {
		http.Redirect(w, r, "/login?error=oauth", http.StatusTemporaryRedirect)
		return
	}

	Login(w, r, int(userID), username)
	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}
