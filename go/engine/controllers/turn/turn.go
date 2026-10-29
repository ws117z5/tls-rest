// Package turn is the reusable TURN/STUN controller, backing papers' video
// mesh; any module can call FetchCredentials/GetIceServers.
package turn

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	config "tls-rest/go/app/constants"
	"tls-rest/go/engine/controllers/db/cache"
	"tls-rest/go/engine/controllers/functions"
	"tls-rest/go/engine/controllers/module"

	"github.com/gorilla/mux"
)

// coturnCredentialTTL is how long a minted coturn username/password pair
// stays valid (embedded in the username itself, per the TURN REST API
// convention coturn's use-auth-secret implements).
const coturnCredentialTTL = 2 * time.Hour

// MintCoturnCredentials mints a time-limited TURN REST API credential pair
// against the self-hosted coturn instance for the given calling module
// (e.g. "papers", "p2pmsg") and user — coturn has one shared realm/secret, so
// the module tag travels in the username itself for later log filtering.
func MintCoturnCredentials(callingModule string, userID int) map[string]any {
	expires := time.Now().Add(coturnCredentialTTL).Unix()
	username := fmt.Sprintf("%d:%s:%d", expires, callingModule, userID)
	mac := hmac.New(sha1.New, []byte(config.TurnSharedSecret))
	mac.Write([]byte(username))
	password := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	host := config.TurnHost
	return map[string]any{
		"urls": []string{
			"stun:" + host + ":3478",
			"turn:" + host + ":3478?transport=udp",
			"turn:" + host + ":3478?transport=tcp",
			"turns:" + host + ":5349?transport=tcp",
		},
		"username":   username,
		"credential": password,
	}
}

// Fallbacks when go.config.json's "turn" section is unset.
const (
	defaultCredentialsURL = "https://rtc.live.cloudflare.com/v1/turn/keys/{keyID}/credentials/generate"
	defaultTTLSeconds     = 86400
)

var defaultFallbackStunURLs = []string{"stun:stun.l.google.com:19302"}

// FallbackICEServers is the public-STUN-only list used when TURN is unconfigured.
func FallbackICEServers() []map[string]any {
	urls := config.Config.Turn.FallbackStunURLs
	if len(urls) == 0 {
		urls = defaultFallbackStunURLs
	}
	return []map[string]any{{"urls": urls}}
}

// credentialsURL substitutes CF_TURN_TOKEN_ID into go.config.json's "turn.credentialsUrl".
func credentialsURL() string {
	tmpl := config.Config.Turn.CredentialsURL
	if tmpl == "" {
		tmpl = defaultCredentialsURL
	}
	return strings.ReplaceAll(tmpl, "{keyID}", config.CFTurnKeyID)
}

// ttlSeconds is go.config.json's "turn.ttlSeconds".
func ttlSeconds() int {
	if config.Config.Turn.TTLSeconds > 0 {
		return config.Config.Turn.TTLSeconds
	}
	return defaultTTLSeconds
}

// cfTurnCredentialsResponse is Cloudflare's POST .../credentials/generate response.
type cfTurnCredentialsResponse struct {
	IceServers struct {
		Urls       []string `json:"urls"`
		Username   string   `json:"username"`
		Credential string   `json:"credential"`
	} `json:"iceServers"`
}

// GetIceServers returns the ICE server list for an authenticated session's
// RTCPeerConnection. Served at GET /api/config/ice. The optional "module"
// query param (e.g. "papers", "p2pmsg") is only used by the coturn path, to
// tag the minted credential's username for later log filtering.
func GetIceServers(w http.ResponseWriter, r *http.Request) {
	s := cache.SessionFromContext(r.Context())
	if s == nil || s.UserID <= 0 {
		functions.JSONError(w, http.StatusUnauthorized, "no session")
		return
	}

	if config.TurnSharedSecret != "" {
		callingModule := r.URL.Query().Get("module")
		if callingModule == "" {
			callingModule = "unknown"
		}
		functions.WriteJSON(w, http.StatusOK, map[string]any{
			"iceServers": []map[string]any{MintCoturnCredentials(callingModule, s.UserID)},
		})
		return
	}

	if config.CFTurnKeyID == "" || config.CFTurnToken == "" || CapExceeded() {
		functions.WriteJSON(w, http.StatusOK, map[string]any{"iceServers": FallbackICEServers()})
		return
	}

	servers, err := FetchCredentials()
	if err != nil {
		functions.WriteJSON(w, http.StatusOK, map[string]any{"iceServers": FallbackICEServers()})
		return
	}
	functions.WriteJSON(w, http.StatusOK, map[string]any{"iceServers": servers})
}

// FetchCredentials generates one set of time-limited TURN credentials via the
// configured TURN provider's credentials API.
func FetchCredentials() ([]map[string]any, error) {
	url := credentialsURL()
	body := fmt.Sprintf(`{"ttl":%d}`, ttlSeconds())
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+config.CFTurnToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("turn: credentials request status %d", resp.StatusCode)
	}

	var cf cfTurnCredentialsResponse
	if err := json.NewDecoder(resp.Body).Decode(&cf); err != nil {
		return nil, err
	}
	return []map[string]any{
		{"urls": cf.IceServers.Urls, "username": cf.IceServers.Username, "credential": cf.IceServers.Credential},
	}, nil
}

// init registers the /api/config/ice route. Routed
// directly on the root router (not under any module's /{id}-bearing
// subrouter) so it can never lose a match to a module's generic view route.
func init() {
	module.AddRouteRegistrar(func(router *mux.Router) {
		router.HandleFunc("/api/config/ice", GetIceServers).Methods("GET")
	})
}
