// TURN credentials for the papers video mesh (see js/src/pages/papers/lib/
// mesh.ts): peers behind symmetric NAT or restrictive firewalls can't
// complete a direct P2P connection with STUN alone and need a relay. Rather
// than self-host one (which would need a stable public IP the workstation
// doesn't have), this generates short-lived credentials from Cloudflare's
// managed TURN service.
package papers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	config "tls-rest/go/constants"
	"tls-rest/go/engine/controllers/db/cache"
	"tls-rest/go/engine/controllers/functions"
)

// fallbackIceServers is what a client gets when Cloudflare TURN isn't
// configured (CF_TURN_KEY_ID/CF_TURN_TOKEN unset) — public STUN only, the
// same config the mesh always used before this existed. Works for direct P2P;
// not behind symmetric NAT.
var fallbackIceServers = []map[string]any{
	{"urls": []string{"stun:stun.l.google.com:19302"}},
}

// cfTurnCredentialsResponse is Cloudflare's response shape from
// POST /v1/turn/keys/{key_id}/credentials/generate.
type cfTurnCredentialsResponse struct {
	IceServers struct {
		Urls       []string `json:"urls"`
		Username   string   `json:"username"`
		Credential string   `json:"credential"`
	} `json:"iceServers"`
}

// GetIceServers returns the ICE server list a client's RTCPeerConnection
// should use. GET /papers/ice-config
func GetIceServers(w http.ResponseWriter, r *http.Request) {
	s := cache.SessionFromContext(r.Context())
	if s == nil || s.UserID <= 0 {
		functions.JSONError(w, http.StatusUnauthorized, "no session")
		return
	}

	if config.CFTurnKeyID == "" || config.CFTurnToken == "" || turnCapExceeded() {
		functions.WriteJSON(w, http.StatusOK, map[string]any{"iceServers": fallbackIceServers})
		return
	}

	servers, err := fetchCloudflareTurnCredentials()
	if err != nil {
		functions.WriteJSON(w, http.StatusOK, map[string]any{"iceServers": fallbackIceServers})
		return
	}
	functions.WriteJSON(w, http.StatusOK, map[string]any{"iceServers": servers})
}

// fetchCloudflareTurnCredentials generates one set of time-limited TURN
// credentials (24h TTL) via Cloudflare's Realtime TURN API.
func fetchCloudflareTurnCredentials() ([]map[string]any, error) {
	url := "https://rtc.live.cloudflare.com/v1/turn/keys/" + config.CFTurnKeyID + "/credentials/generate"
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(`{"ttl":86400}`))
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
		return nil, fmt.Errorf("cloudflare turn: status %d", resp.StatusCode)
	}

	var cf cfTurnCredentialsResponse
	if err := json.NewDecoder(resp.Body).Decode(&cf); err != nil {
		return nil, err
	}
	return []map[string]any{
		{"urls": cf.IceServers.Urls, "username": cf.IceServers.Username, "credential": cf.IceServers.Credential},
	}, nil
}
