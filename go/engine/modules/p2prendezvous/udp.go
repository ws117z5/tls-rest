package p2prendezvous

import (
	"fmt"
	"net"

	config "tls-rest/go/app/constants"
	"tls-rest/go/engine/controllers/log"
)

// heartbeatPacketSize is the exact size of a valid heartbeat: a 32-byte
// token, hex-encoded to 64 ASCII bytes (see tokens.go). Anything else is
// silently dropped rather than parsed — this listener only ever needs to
// understand one packet shape. The actual peer-to-peer traffic (hole-punch
// probes, the DH handshake, encrypted messages) never touches the server at
// all; it goes directly between clients once each has the other's address.
const heartbeatPacketSize = 64

// ackByte is the whole reply to a valid heartbeat — just enough for the
// client to know its registration landed. No reply is sent for an invalid
// token, so this listener can't be used as a spoofed-source reflection
// oracle (and the 64-byte-in/1-byte-out shape rules out amplification even
// if it were).
var ackByte = []byte{1}

// startUDPListener runs until the process exits; a bind failure is logged
// and swallowed rather than crashing the whole server — P2P messaging being
// unavailable shouldn't take the web app down with it.
func startUDPListener() {
	port := config.Config.UDPPort()
	addr := fmt.Sprintf(":%d", port)
	conn, err := net.ListenPacket("udp", addr)
	if err != nil {
		log.Errorf("p2prendezvous: could not open UDP listener on %s: %v", addr, err)
		return
	}
	defer conn.Close()
	log.Infof("p2prendezvous: UDP rendezvous listener on %s", addr)

	buf := make([]byte, 512)
	for {
		n, remote, err := conn.ReadFrom(buf)
		if err != nil {
			log.Errorf("p2prendezvous: UDP read error: %v", err)
			continue
		}
		if n != heartbeatPacketSize {
			continue
		}
		udpAddr, ok := remote.(*net.UDPAddr)
		if !ok {
			continue
		}

		token := string(buf[:n])
		userID, ok := verifyToken(token)
		if !ok {
			continue
		}

		setAddr(userID, udpAddr)
		_, _ = conn.WriteTo(ackByte, remote)
	}
}
