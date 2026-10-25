package p2prendezvous

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
)

// rooms let any number of authenticated users exchange messages with each
// other WITHOUT being friends — unlike the friends-gated 1:1 connect flow in
// handlers.go, knowing/choosing a room's code is the only authorization a
// room needs. Membership is in-memory only, same as presence: a room exists
// for as long as it has at least one member and is forgotten once empty.
// There is no membership expiry tied to presence going stale — a client
// that crashes without calling leave stays "in" the room until it does, or
// until the room happens to empty out some other way. Harmless in practice
// (every place membership is read already filters to who's currently
// online — see joinRoomAndRespond), just not instant cleanup.
var rooms = struct {
	mu        sync.Mutex
	members   map[string]map[int]bool // room code -> set of member user ids
	userRooms map[int]map[string]bool // user id -> set of room codes they're in
}{
	members:   map[string]map[int]bool{},
	userRooms: map[int]map[string]bool{},
}

// signalRoomPrefix namespaces mesh signal "room" keys used for room
// membership pushes away from the fixed signalRoom the 1:1 DM flow uses, so
// a user-chosen room code can never collide with it.
const signalRoomPrefix = "room:"

func generateRoomCode() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// joinRoom adds userID to code's membership (creating the room if this is
// its first member) and returns every other current member's id.
func joinRoom(code string, userID int) []int {
	rooms.mu.Lock()
	defer rooms.mu.Unlock()

	m, ok := rooms.members[code]
	if !ok {
		m = map[int]bool{}
		rooms.members[code] = m
	}
	others := make([]int, 0, len(m))
	for id := range m {
		others = append(others, id)
	}
	m[userID] = true

	ur, ok := rooms.userRooms[userID]
	if !ok {
		ur = map[string]bool{}
		rooms.userRooms[userID] = ur
	}
	ur[code] = true

	return others
}

// leaveRoom removes userID from code's membership, forgetting the room
// entirely once it's empty, and returns who's left.
func leaveRoom(code string, userID int) []int {
	rooms.mu.Lock()
	defer rooms.mu.Unlock()

	if ur := rooms.userRooms[userID]; ur != nil {
		delete(ur, code)
		if len(ur) == 0 {
			delete(rooms.userRooms, userID)
		}
	}

	m, ok := rooms.members[code]
	if !ok {
		return nil
	}
	delete(m, userID)
	remaining := make([]int, 0, len(m))
	for id := range m {
		remaining = append(remaining, id)
	}
	if len(m) == 0 {
		delete(rooms.members, code)
	}
	return remaining
}

// roomsForUser returns every room code userID is currently a member of, so
// the SSE stream handler knows which mesh signal queues to drain.
func roomsForUser(userID int) []string {
	rooms.mu.Lock()
	defer rooms.mu.Unlock()
	ur := rooms.userRooms[userID]
	out := make([]string, 0, len(ur))
	for code := range ur {
		out = append(out, code)
	}
	return out
}
