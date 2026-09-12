package papers

import (
	"encoding/json"
	"errors"
	"time"

	"tls-rest/go/engine/controllers/db/cache"
)

// errCacheMiss makes an in-memory-only cache report a miss (no DB backing).
var errCacheMiss = errors.New("papers: not cached")

// Two dedicated caches, separate from the session cache:
//
//   gameUserCache  — a player's chosen game name, keyed by their SESSION key, so
//                    it persists across rooms and pre-fills the name input. It is
//                    NOT stored on the users table (game identity ≠ account).
//   roomStateCache — live room state (players + their negotiator params + turn
//                    state), keyed by room UUID, for fast per-room access.

// GameUser is a player's game identity + word for a room round.
type GameUser struct {
	SessionKey string `json:"session_key"`
	Name       string `json:"name"`
	Word       string `json:"word"`
}

// RoomState is the fast-access live state for one room.
type RoomState struct {
	RoomUUID  string              `json:"room_uuid"`
	Order     []string            `json:"order"`     // player session keys, in join order (turn order)
	ActiveIx  int                 `json:"active_ix"` // index into Order whose turn it is
	Started   bool                `json:"started"`
	Wrong     int                 `json:"wrong"` // wrong clicks for the active player this turn
	Deadline  time.Time           `json:"deadline"`
	Players   map[string]GameUser `json:"players"`    // session key -> game identity
	Assigned  map[string]string   `json:"assigned"`   // session key -> word they wear (from another player)
	NegParams map[string]any      `json:"neg_params"` // session key -> negotiator params (opaque)
	// Finished is the scoreboard: session keys in the order they correctly
	// guessed their word (self-reported — the server can't verify a word
	// spoken aloud over video). Index 0 is the first to guess. A finished
	// player is skipped when picking the next active player.
	Finished []string `json:"finished"`
}

var (
	// Keyed by session key. Backed by an optional game_users table (getter/setter
	// below); swap in real DB IO if you want durability, otherwise it's in-memory.
	GameUserCache = cache.NewCache[GameUser](
		func(string) (GameUser, error) { return GameUser{}, errCacheMiss },
		func(string, GameUser) error { return nil },
	).WithTTL(30 * 24 * time.Hour)

	// Keyed by room UUID. Purely in-memory (fast path); rebuilt from papers.users
	// on miss if you wire a getter.
	RoomStateCache = cache.NewCache[RoomState](
		func(string) (RoomState, error) { return RoomState{}, errCacheMiss },
		func(string, RoomState) error { return nil },
	).WithTTL(6 * time.Hour)
)

// helpers ------------------------------------------------------------------

func GetGameUser(sessionKey string) (GameUser, bool) {
	if v, err := GameUserCache.Get(sessionKey); err == nil && v != nil {
		return *v, true
	}
	return GameUser{}, false
}

func SetGameUser(u GameUser) {
	GameUserCache.Set(u.SessionKey, u)
}

func GetRoomState(roomUUID string) (RoomState, bool) {
	if v, err := RoomStateCache.Get(roomUUID); err == nil && v != nil {
		return *v, true
	}
	return RoomState{RoomUUID: roomUUID, Players: map[string]GameUser{}, Assigned: map[string]string{}, NegParams: map[string]any{}}, false
}

func SetRoomState(st RoomState) {
	RoomStateCache.Set(st.RoomUUID, st)
}

// marshalUsers renders the room's players for storage on papers.users.
func (st RoomState) marshalUsers() string {
	b, _ := json.Marshal(st.Players)
	return string(b)
}
