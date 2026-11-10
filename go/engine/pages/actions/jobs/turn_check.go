// TURN reachability check, backing the "turn_check" Action.
package jobs

import (
	"fmt"
	"net"
	"strings"
	"time"

	pionturn "github.com/pion/turn/v5"

	config "tls-rest/go/app/constants"
	actionsctl "tls-rest/go/engine/controllers/actions"
	"tls-rest/go/engine/controllers/field"
	"tls-rest/go/engine/controllers/functions"
	"tls-rest/go/engine/controllers/turn"
)

const allocateTimeout = 5 * time.Second

// checkTurnReachability mints fresh coturn credentials for serverHost and performs a real TURN
// relay allocation against it — the self-hosted coturn path (TURN_HOST/TURN_SHARED_SECRET), not
// Cloudflare's managed TURN, since that's the one GetIceServers actually hands to real clients
// whenever TURN_SHARED_SECRET is set (see turn.go's GetIceServers: coturn is tried first).
func checkTurnReachability(params map[string]any) (string, error) {
	serverHost := strings.TrimSpace(functions.Coerce[string](params["server_url"]))
	if serverHost == "" {
		serverHost = config.TurnHost
	}
	if config.TurnSharedSecret == "" {
		return "", fmt.Errorf("coturn not configured (TURN_SHARED_SECRET unset)")
	}

	creds := turn.MintCoturnCredentialsForHost(serverHost, "turn_check", 0)
	username, _ := creds["username"].(string)
	credential, _ := creds["credential"].(string)
	urls, _ := creds["urls"].([]string)

	addr, err := firstTurnAddr(urls)
	if err != nil {
		return "", err
	}

	conn, err := net.ListenPacket("udp4", "0.0.0.0:0")
	if err != nil {
		return "", fmt.Errorf("opening local socket: %w", err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(allocateTimeout)); err != nil {
		return "", err
	}

	client, err := pionturn.NewClient(&pionturn.ClientConfig{
		STUNServerAddr: addr,
		TURNServerAddr: addr,
		Conn:           conn,
		Username:       username,
		Password:       credential,
	})
	if err != nil {
		return "", fmt.Errorf("creating TURN client: %w", err)
	}
	defer client.Close()

	if err := client.Listen(); err != nil {
		return "", fmt.Errorf("listen: %w", err)
	}

	relayConn, err := client.Allocate()
	if err != nil {
		return "", fmt.Errorf("allocate against %s: %w", addr, err)
	}
	defer relayConn.Close()

	return fmt.Sprintf("TURN OK: allocated relay %s via %s", relayConn.LocalAddr(), addr), nil
}

// firstTurnAddr returns the host:port of the first plain "turn:" URL.
func firstTurnAddr(urls []string) (string, error) {
	for _, u := range urls {
		if !strings.HasPrefix(u, "turn:") {
			continue
		}
		rest := strings.TrimPrefix(u, "turn:")
		if i := strings.IndexByte(rest, '?'); i >= 0 {
			rest = rest[:i]
		}
		return rest, nil
	}
	return "", fmt.Errorf("no turn: URL in ICE server list (got %v)", urls)
}

func init() {
	actionsctl.Register(&actionsctl.Action{
		ID:          "turn_check",
		Name:        "TURN reachability check",
		Description: "Mints fresh coturn credentials and performs a real relay allocation to confirm the TURN server is reachable and the credentials work.",
		Params: []field.Field{
			field.NewField("server_url", field.TYPE_STRING, false).
				WithLabel("TURN server URL").WithDefault(config.TurnHost),
		},
		RunFunc: checkTurnReachability,
	})
}
