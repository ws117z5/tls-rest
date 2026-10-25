package p2prendezvous

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// tokenTTL is how long a minted rendezvous token is accepted over UDP. The
// client refreshes it (POST /api/p2p/token again) well before expiry — see
// p2p/RendezvousClient.kt. Kept short and UDP-only on purpose: this token is
// what stands in for the client's real, long-lived bearer token on the one
// channel (UDP) that can't carry an Authorization header or TLS.
const tokenTTL = 2 * time.Minute

type tokenEntry struct {
	userID  int
	expires time.Time
}

var tokens = struct {
	mu      sync.Mutex
	byToken map[string]tokenEntry
}{byToken: map[string]tokenEntry{}}

// mintToken generates a fresh random token for userID, valid for tokenTTL.
// Minting again for the same user simply adds a second valid token rather
// than invalidating the first — harmless, since both still only ever
// resolve back to the same userID, and the old one expires on its own.
func mintToken(userID int) string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	token := hex.EncodeToString(b)

	tokens.mu.Lock()
	defer tokens.mu.Unlock()
	tokens.byToken[token] = tokenEntry{userID: userID, expires: time.Now().Add(tokenTTL)}
	return token
}

// verifyToken resolves a UDP heartbeat's token to a userID, opportunistically
// sweeping expired entries while it holds the lock.
func verifyToken(token string) (int, bool) {
	tokens.mu.Lock()
	defer tokens.mu.Unlock()

	now := time.Now()
	for t, e := range tokens.byToken {
		if now.After(e.expires) {
			delete(tokens.byToken, t)
		}
	}

	e, ok := tokens.byToken[token]
	if !ok || now.After(e.expires) {
		return 0, false
	}
	return e.userID, true
}
