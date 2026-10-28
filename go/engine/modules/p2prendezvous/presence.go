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

// IsOnline is lookup without the address — exported for go/modules/msgusers'
// admin "online" column/filter, in addition to this package's own use for
// building online-friends snapshots.
func IsOnline(userID int) bool {
	_, ok := lookup(userID)
	return ok
}
