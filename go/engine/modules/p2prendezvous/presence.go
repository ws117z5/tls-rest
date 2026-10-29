package p2prendezvous

import (
	"net"
	"sync"
	"time"

	"tls-rest/go/engine/controllers/db/pgdb"
)

// onlineWindow is how long a UDP heartbeat keeps a user "online" after it
// arrives. Clients are expected to heartbeat well inside this window (see
// udp.go) — see p2p/UdpLink.kt on the Android side. The Android heartbeat is
// every 20s (P2PManager.kt's HEARTBEAT_INTERVAL_SEC), so 30s covers it with
// slack without the admin "online" filter (go/modules/msgusers) going stale.
const onlineWindow = 30 * time.Second

type presenceEntry struct {
	addr     *net.UDPAddr
	lastSeen time.Time
	// tcpAddr is this user's externally-observed address on the TCP registration
	// listener (tcp.go) — a separate NAT mapping from addr/lastSeen above (different
	// protocol, different local port on the client), tracked with its own staleness
	// window since a client can be UDP-online without ever having registered one yet.
	tcpAddr     *net.TCPAddr
	tcpLastSeen time.Time
}

// presence is the in-memory "who's online, and where" registry this whole
// package exists to maintain. Never persisted — it's rebuilt from client
// heartbeats as they arrive, and a restart just means every client
// re-registers on its next heartbeat.
var presence = struct {
	mu     sync.Mutex
	byUser map[int]*presenceEntry
}{byUser: map[int]*presenceEntry{}}

// setAddr records userID's most recently observed UDP source address — the
// server's own view of "where this client is", which is what makes NAT
// traversal work without a separate STUN step.
func setAddr(userID int, addr *net.UDPAddr) {
	presence.mu.Lock()
	defer presence.mu.Unlock()
	presence.byUser[userID] = &presenceEntry{addr: addr, lastSeen: time.Now()}
	go persistLastSeen(userID)
}

// persistLastSeen durably records "last seen" (users.last_seen) so a chat
// partner who's shown it (go/engine/modules/chats' loadChat, gated by
// show_online_status) can still see it after this process restarts and the
// in-memory presence registry above resets. Fire-and-forget off the UDP read
// loop's own goroutine — a dropped write just means a slightly stale
// timestamp until the next heartbeat, not a user-visible error.
func persistLastSeen(userID int) {
	db, err := pgdb.GetInstance()
	if err != nil {
		return
	}
	_, _ = db.Exec(`UPDATE users SET last_seen = now() WHERE id = $1`, userID)
}

// lookup returns userID's current address, only if it has heartbeated
// within onlineWindow — a stale entry counts as offline.
func lookup(userID int) (*net.UDPAddr, bool) {
	presence.mu.Lock()
	defer presence.mu.Unlock()
	e, ok := presence.byUser[userID]
	if !ok || time.Since(e.lastSeen) > onlineWindow {
		return nil, false
	}
	return e.addr, true
}

// setTCPAddr records userID's most recently observed TCP registration source address (tcp.go)
// — a NAT mapping the real P2P TCP connection can actually use, unlike the UDP one above, which
// reflects a different local port/protocol and has no relationship to a TCP flow's own mapping.
func setTCPAddr(userID int, addr *net.TCPAddr) {
	presence.mu.Lock()
	defer presence.mu.Unlock()
	e, ok := presence.byUser[userID]
	if !ok {
		e = &presenceEntry{}
		presence.byUser[userID] = e
	}
	e.tcpAddr = addr
	e.tcpLastSeen = time.Now()
}

// lookupTCP returns userID's current TCP registration address, only if one was ever recorded
// and it's still within onlineWindow — a user who's UDP-online but never registered over TCP
// (or whose registration went stale) has none.
func lookupTCP(userID int) (*net.TCPAddr, bool) {
	presence.mu.Lock()
	defer presence.mu.Unlock()
	e, ok := presence.byUser[userID]
	if !ok || e.tcpAddr == nil || time.Since(e.tcpLastSeen) > onlineWindow {
		return nil, false
	}
	return e.tcpAddr, true
}

// IsOnline is lookup without the address — exported for go/modules/msgusers'
// admin "online" column/filter, in addition to this package's own use for
// building online-friends snapshots.
func IsOnline(userID int) bool {
	_, ok := lookup(userID)
	return ok
}

// Addr returns userID's last-heartbeat public IP (no port) — exported for
// go/modules/msgusers' admin "IP" column, e.g. to spot two users sharing one
// NAT (same home network, no hairpin, hence no direct P2P between them).
func Addr(userID int) (string, bool) {
	addr, ok := lookup(userID)
	if !ok {
		return "", false
	}
	return addr.IP.String(), true
}
