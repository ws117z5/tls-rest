package p2prendezvous

import (
	"net"
	"sync"
	"time"
)

// onlineWindow is how long a UDP heartbeat keeps a user "online" after it
// arrives. Clients are expected to heartbeat well inside this window (see
// udp.go) — see p2p/UdpLink.kt on the Android side.
const onlineWindow = 45 * time.Second

type presenceEntry struct {
	addr     *net.UDPAddr
	lastSeen time.Time
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

// isOnline is lookup without the address, for building online-friends snapshots.
func isOnline(userID int) bool {
	_, ok := lookup(userID)
	return ok
}
