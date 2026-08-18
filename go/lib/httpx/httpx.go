// Package httpx derives absolute, per-request URLs so the app can be reached on
// several origins (localhost, LAN, public domain) without any hardcoded host.
//
// The rule: URLs the browser hits on the same origin stay RELATIVE (no host
// needed). This package is only for values that must be absolute because they
// leave the app — OAuth redirect URIs, links in emails, canonical links — and
// for reflecting a safe CORS Origin.
//
// Everything is validated against a trusted-host allowlist (SetTrustedHosts,
// loaded from the APP_HOSTS env at startup). That defeats Host-header spoofing:
// a request arriving with an unknown/forged Host never steers a derived URL to
// an attacker; it falls back to the canonical (first) trusted host.
package httpx

import (
	"net"
	"net/http"
	"strings"
	"sync"
)

var (
	mu           sync.RWMutex
	trustedHosts []string // hostnames only, no port; first entry is canonical
)

// SetTrustedHosts installs the allowlist. Call once at startup, e.g.
//
//	httpx.SetTrustedHosts(strings.Split(constants.Env("APP_HOSTS", "localhost"), ","))
//
// Entries are hostnames without a port ("localhost", "192.168.1.50",
// "example.com"). Blank entries are ignored.
func SetTrustedHosts(hosts []string) {
	cleaned := make([]string, 0, len(hosts))
	for _, h := range hosts {
		if h = strings.TrimSpace(h); h != "" {
			cleaned = append(cleaned, h)
		}
	}
	mu.Lock()
	trustedHosts = cleaned
	mu.Unlock()
}

func hostsSnapshot() []string {
	mu.RLock()
	defer mu.RUnlock()
	return trustedHosts
}

// Scheme returns the external scheme, honouring a terminating proxy that sets
// X-Forwarded-Proto (Cloudflare/Heroku/nginx). Falls back to r.TLS for direct
// connections (e.g. on the LAN with no proxy).
func Scheme(r *http.Request) string {
	if p := r.Header.Get("X-Forwarded-Proto"); p != "" {
		return strings.ToLower(strings.TrimSpace(strings.Split(p, ",")[0]))
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

// Host returns the external host[:port], honouring a terminating proxy that
// sets X-Forwarded-Host, and validated against the allowlist. An unknown or
// spoofed host resolves to the canonical trusted host instead.
func Host(r *http.Request) string {
	h := r.Header.Get("X-Forwarded-Host")
	if h == "" {
		h = r.Host
	}
	h = strings.TrimSpace(strings.Split(h, ",")[0])

	name := h
	if hn, _, err := net.SplitHostPort(h); err == nil {
		name = hn
	}

	hosts := hostsSnapshot()
	for _, allowed := range hosts {
		if strings.EqualFold(name, allowed) {
			return h // trusted → use exactly what the client presented
		}
	}
	if len(hosts) > 0 {
		return hosts[0] // untrusted/forged → canonical host, never the attacker's
	}
	return h // allowlist unset (dev) → best effort
}

// BaseURL is the absolute origin for this request: scheme://host[:port], no
// trailing slash. Compose absolute paths from it, e.g.
//
//	httpx.BaseURL(r) + "/users/Auth/GoogleCallback"
func BaseURL(r *http.Request) string {
	return Scheme(r) + "://" + Host(r)
}

// AllowOrigin reflects the request's Origin header onto the response, but only
// when its host is allowlisted — the correct pattern for credentialed
// (cookie-bearing) requests, where "*" is both unsafe and rejected by browsers.
// No header is written for an untrusted/absent Origin. Safe to call before
// writing the body; returns whether an origin was allowed.
func AllowOrigin(w http.ResponseWriter, r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return false
	}

	name := origin
	if i := strings.Index(name, "://"); i >= 0 {
		name = name[i+3:]
	}
	if hn, _, err := net.SplitHostPort(name); err == nil {
		name = hn
	}

	for _, allowed := range hostsSnapshot() {
		if strings.EqualFold(name, allowed) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			return true
		}
	}
	return false
}
