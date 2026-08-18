package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	config "tls-rest/go/constants"
	"tls-rest/go/engine/modules/users"
	"tls-rest/go/lib"
	"tls-rest/go/lib/httpx"

	"golang.org/x/oauth2"
)

// googleEndpoint is Google's OAuth2 endpoint, inlined so we don't pull in
// golang.org/x/oauth2/google (which drags in cloud.google.com/go/compute/
// metadata) merely for a constant. URLs are Google's stable OAuth endpoints.
var googleEndpoint = oauth2.Endpoint{
	AuthURL:  "https://accounts.google.com/o/oauth2/auth",
	TokenURL: "https://oauth2.googleapis.com/token",
}

// providerDef describes one OAuth provider generically: its endpoint, scopes,
// credential getters, and a fetch func that normalizes the provider's user-info
// into a users.OAuthAccount. Adding a provider is adding one entry here.
type providerDef struct {
	name     string          // canonical key, lower-case: "google", "github", ...
	segment  string          // URL segment for routes/redirect: "Google" -> /users/Auth/GoogleCallback
	endpoint oauth2.Endpoint // authorize + token URLs
	scopes   []string
	id       func() string
	secret   func() string
	fetch    func(ctx context.Context, cfg *oauth2.Config, tok *oauth2.Token) (*users.OAuthAccount, error)
}

// providers is the registry, keyed by canonical (lower-case) name.
var providers = map[string]*providerDef{
	"google": {
		name:     "google",
		segment:  "Google",
		endpoint: googleEndpoint,
		scopes:   []string{"https://www.googleapis.com/auth/userinfo.email", "profile", "email"},
		id:       func() string { return config.GoogleID },
		secret:   func() string { return config.GoogleSecret },
		fetch:    fetchGoogle,
	},
	"github": {
		name:     "github",
		segment:  "Github",
		endpoint: oauth2.Endpoint{AuthURL: "https://github.com/login/oauth/authorize", TokenURL: "https://github.com/login/oauth/access_token"},
		scopes:   []string{"read:user", "user:email"},
		id:       func() string { return config.GithubID },
		secret:   func() string { return config.GithubSecret },
		fetch:    fetchGithub,
	},
	"facebook": {
		name:     "facebook",
		segment:  "Facebook",
		endpoint: oauth2.Endpoint{AuthURL: "https://www.facebook.com/v19.0/dialog/oauth", TokenURL: "https://graph.facebook.com/v19.0/oauth/access_token"},
		scopes:   []string{"public_profile", "email"},
		id:       func() string { return config.FacebookID },
		secret:   func() string { return config.FacebookSecret },
		fetch:    fetchFacebook,
	},
	"vk": {
		name:     "vk",
		segment:  "Vk",
		endpoint: oauth2.Endpoint{AuthURL: "https://oauth.vk.com/authorize", TokenURL: "https://oauth.vk.com/access_token"},
		scopes:   []string{"email"},
		id:       func() string { return strconv.Itoa(config.VKID) },
		secret:   func() string { return config.VKSecKey },
		fetch:    fetchVK,
	},
}

// config builds the per-request OAuth2 config, deriving the redirect URI from
// the host the client actually used (httpx.BaseURL, allowlisted) so one build
// serves localhost, LAN and the public domain. Each host used must be
// registered as an authorized redirect URI in the provider's console.
func (p *providerDef) config(r *http.Request) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     p.id(),
		ClientSecret: p.secret(),
		Endpoint:     p.endpoint,
		RedirectURL:  httpx.BaseURL(r) + "/users/Auth/" + p.segment + "Callback",
		Scopes:       p.scopes,
	}
}

func (p *providerDef) configured() bool { return p.id() != "" && p.secret() != "" }

// --- CSRF state (per-request, cookie-bound) ---------------------------------
//
// Replaces the former single process-wide state string. A fresh random state is
// generated per login, stored in a short-lived cookie, and compared on the
// callback — so concurrent logins (across providers/users) can't collide and a
// forged callback without the matching cookie is rejected. SameSite=Lax lets
// the cookie ride the top-level redirect back from the provider.

const stateCookie = "oauth_state"

func setStateCookie(w http.ResponseWriter, r *http.Request) string {
	state, _ := lib.GetRandomHash(16)
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookie,
		Value:    state,
		Path:     "/",
		Expires:  time.Now().Add(10 * time.Minute),
		HttpOnly: true,
		Secure:   httpx.Scheme(r) == "https",
		SameSite: http.SameSiteLaxMode,
	})
	return state
}

func checkStateCookie(w http.ResponseWriter, r *http.Request) bool {
	// Clear it regardless of outcome (single use).
	defer http.SetCookie(w, &http.Cookie{Name: stateCookie, Value: "", Path: "/", MaxAge: -1})

	c, err := r.Cookie(stateCookie)
	if err != nil || c.Value == "" {
		return false
	}
	got := r.FormValue("state")
	return got != "" && got == c.Value
}

// --- Provider account fetchers ----------------------------------------------

// apiGetJSON performs an authenticated GET (client already injects the bearer
// token) with optional extra headers and decodes JSON into out.
func apiGetJSON(ctx context.Context, client *http.Client, url string, headers map[string]string, out interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s: unexpected status %d", url, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func fetchGoogle(ctx context.Context, cfg *oauth2.Config, tok *oauth2.Token) (*users.OAuthAccount, error) {
	client := cfg.Client(ctx, tok)
	var g struct {
		ID        string `json:"id"`
		Email     string `json:"email"`
		FirstName string `json:"given_name"`
		LastName  string `json:"family_name"`
		Picture   string `json:"picture"`
	}
	if err := apiGetJSON(ctx, client, "https://www.googleapis.com/oauth2/v2/userinfo", nil, &g); err != nil {
		return nil, err
	}
	return &users.OAuthAccount{
		ProviderID: g.ID,
		Email:      g.Email,
		FirstName:  g.FirstName,
		LastName:   g.LastName,
		Image:      g.Picture,
	}, nil
}

func fetchGithub(ctx context.Context, cfg *oauth2.Config, tok *oauth2.Token) (*users.OAuthAccount, error) {
	client := cfg.Client(ctx, tok)
	headers := map[string]string{"Accept": "application/vnd.github+json", "User-Agent": "tls-rest"}

	var u struct {
		ID        int64  `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		AvatarURL string `json:"avatar_url"`
		Email     string `json:"email"`
	}
	if err := apiGetJSON(ctx, client, "https://api.github.com/user", headers, &u); err != nil {
		return nil, err
	}

	email := u.Email
	if email == "" {
		// Primary email can be private on /user; fetch it explicitly.
		var emails []struct {
			Email    string `json:"email"`
			Primary  bool   `json:"primary"`
			Verified bool   `json:"verified"`
		}
		if err := apiGetJSON(ctx, client, "https://api.github.com/user/emails", headers, &emails); err == nil {
			for _, e := range emails {
				if e.Primary && e.Verified {
					email = e.Email
					break
				}
			}
		}
	}

	first, last := splitName(u.Name)
	return &users.OAuthAccount{
		ProviderID: strconv.FormatInt(u.ID, 10),
		Email:      email,
		FirstName:  first,
		LastName:   last,
		Image:      u.AvatarURL,
		Username:   u.Login,
	}, nil
}

func fetchFacebook(ctx context.Context, cfg *oauth2.Config, tok *oauth2.Token) (*users.OAuthAccount, error) {
	client := cfg.Client(ctx, tok)
	var fb struct {
		ID        string `json:"id"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Email     string `json:"email"`
		Picture   struct {
			Data struct {
				URL string `json:"url"`
			} `json:"data"`
		} `json:"picture"`
	}
	url := "https://graph.facebook.com/me?fields=id,first_name,last_name,email,picture.type(large)"
	if err := apiGetJSON(ctx, client, url, nil, &fb); err != nil {
		return nil, err
	}
	return &users.OAuthAccount{
		ProviderID: fb.ID,
		Email:      fb.Email,
		FirstName:  fb.FirstName,
		LastName:   fb.LastName,
		Image:      fb.Picture.Data.URL,
	}, nil
}

func fetchVK(ctx context.Context, cfg *oauth2.Config, tok *oauth2.Token) (*users.OAuthAccount, error) {
	// VK returns the email (when granted) in the token response, not the API.
	email, _ := tok.Extra("email").(string)

	// users.get supplies names + avatar; the numeric id is the stable provider id.
	client := cfg.Client(ctx, tok)
	var vr struct {
		Response []struct {
			ID        int64  `json:"id"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
			Photo200  string `json:"photo_200"`
		} `json:"response"`
	}
	url := "https://api.vk.com/method/users.get?fields=photo_200&v=5.131&access_token=" + tok.AccessToken
	if err := apiGetJSON(ctx, client, url, nil, &vr); err != nil {
		return nil, err
	}
	if len(vr.Response) == 0 {
		return nil, fmt.Errorf("vk: empty users.get response")
	}
	u := vr.Response[0]
	return &users.OAuthAccount{
		ProviderID: strconv.FormatInt(u.ID, 10),
		Email:      email,
		FirstName:  u.FirstName,
		LastName:   u.LastName,
		Image:      u.Photo200,
	}, nil
}

// splitName splits a full display name into first / last on the first space.
func splitName(full string) (first, last string) {
	full = strings.TrimSpace(full)
	if full == "" {
		return "", ""
	}
	if i := strings.IndexByte(full, ' '); i > 0 {
		return full[:i], strings.TrimSpace(full[i+1:])
	}
	return full, ""
}
