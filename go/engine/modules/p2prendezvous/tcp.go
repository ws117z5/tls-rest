package p2prendezvous

import (
	"fmt"
	"io"
	"net"
	"time"

	config "tls-rest/go/app/constants"
	"tls-rest/go/engine/controllers/log"
)

// registrationReadTimeout bounds how long the server waits for a client's token after accepting
// a registration connection — generous for a 64-byte read on an already-established TCP
// connection, but short enough that a client that connects and goes silent doesn't leak a goroutine.
const registrationReadTimeout = 5 * time.Second

// startTCPListener runs until the process exits — same lifecycle as startUDPListener, and a
// bind failure is likewise logged and swallowed rather than crashing the whole server. Separate
// from startUDPListener because this discovers a DIFFERENT NAT mapping (TCP, not UDP) that the
// client's actual peer-to-peer TCP connections depend on — the UDP heartbeat's own mapping has
// no relationship to it (different protocol, different local port on the client).
func startTCPListener() {
	port := config.Config.TCPPort()
	addr := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Errorf("p2prendezvous: could not open TCP registration listener on %s: %v", addr, err)
		return
	}
	defer listener.Close()
	log.Infof("p2prendezvous: TCP registration listener on %s", addr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Errorf("p2prendezvous: TCP accept error: %v", err)
			continue
		}
		go handleRegistration(conn)
	}
}

// handleRegistration reads exactly one heartbeat-shaped token (same format/mint/verify as the
// UDP path — see tokens.go) off a freshly-accepted connection, records the connection's own
// remote address as that user's current TCP NAT mapping, acks, and closes. The client is
// expected to open one of these periodically (like the UDP heartbeat) to keep the mapping warm.
func handleRegistration(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(registrationReadTimeout))

	buf := make([]byte, heartbeatPacketSize)
	n, err := io.ReadFull(conn, buf)
	if err != nil || n != heartbeatPacketSize {
		return
	}

	tcpAddr, ok := conn.RemoteAddr().(*net.TCPAddr)
	if !ok {
		return
	}

	userID, ok := verifyToken(string(buf[:n]))
	if !ok {
		return
	}

	setTCPAddr(userID, tcpAddr)
	_, _ = conn.Write(ackByte)
}
