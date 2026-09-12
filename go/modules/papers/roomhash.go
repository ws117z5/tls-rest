package papers

import (
	"fmt"

	"github.com/ws117z5/mmh3"
)

// roomHashSeed is fixed so a room's public identifier is stable across
// restarts: the same uuid always produces the same hash.
const roomHashSeed uint32 = 0x50415052 // "PAPR"

// hashRoomUUID derives a room's public identifier from its real uuid via
// MurmurHash3 (32-bit), hex-encoded. Computed once at creation (see
// setRoomHash in papers.go) and stored in the `hash` column, so every lookup
// is a plain indexed `WHERE hash = $1` rather than hashing every row on every
// request. This is NOT a security boundary — murmur is not cryptographic and
// the hash is not a secret — the point is that the real uuid never appears in
// a URL or API response, not that the hash is unguessable.
func hashRoomUUID(uuid string) string {
	h, err := mmh3.Hash32(uuid, roomHashSeed)
	if err != nil {
		return ""
	}
	sum := h.AsUint32()
	if len(sum) == 0 {
		return ""
	}
	return fmt.Sprintf("%08x", sum[0])
}
