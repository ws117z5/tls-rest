// TURN reachability check, backing the "turn_check" Action.
package actions

import (
	"fmt"
	"net"
	"strings"
	"time"

	pionturn "github.com/pion/turn/v5"

	config "tls-rest/go/constants"
	actionsctl "tls-rest/go/engine/controllers/actions"
	"tls-rest/go/engine/controllers/turn"
)

const allocateTimeout = 5 * time.Second

// checkTurnReachability performs a real TURN relay allocation with fresh credentials.
func checkTurnReachability() (string, error) {
	if config.CFTurnKeyID == "" || config.CFTurnToken == "" {
		return "", fmt.Errorf("TURN not configured (CF_TURN_TOKEN_ID/CF_TURN_TOKEN_SECRET unset)")
	}

	servers, err := turn.FetchCredentials()
	if err != nil {
		return "", fmt.Errorf("fetching credentials: %w", err)
	}
	if len(servers) == 0 {
		return "", fmt.Errorf("credentials response had no ICE servers")
	}

	username, _ := servers[0]["username"].(string)
	credential, _ := servers[0]["credential"].(string)
	urls, _ := servers[0]["urls"].([]string)

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

func initTurnCheckAction() {
	actionsctl.Register(&actionsctl.Action{
		ID:          "turn_check",
		Name:        "TURN reachability check",
		Description: "Fetches fresh TURN credentials and performs a real relay allocation to confirm the TURN server is reachable and the credentials work.",
		Run:         checkTurnReachability,
	})
}
