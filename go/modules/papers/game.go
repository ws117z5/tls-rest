package papers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"

	"tls-rest/go/engine/controllers/db/cache"
	"tls-rest/go/engine/controllers/db/pgdb"
	"tls-rest/go/engine/controllers/functions"
)

// isRoomCreator reports whether the request's user is the room creator.
func isRoomCreator(r *http.Request, createdBy int64) bool {
	s := cache.SessionFromContext(r.Context())
	return s != nil && int64(s.UserID) == createdBy && createdBy > 0
}

// playerKey identifies a player by their session key (the X-Session-ID cookie or
// bearer token), falling back to the user id. Game names are cached under this
// key, separate from the session cache.
func playerKey(r *http.Request) string {
	if c, err := r.Cookie("X-Session-ID"); err == nil && c.Value != "" {
		return c.Value
	}
	if h := r.Header.Get("Authorization"); h != "" {
		return h
	}
	if s := cache.SessionFromContext(r.Context()); s != nil && s.UserID > 0 {
		return "user:" + strconv.Itoa(s.UserID)
	}
	return ""
}

// roomInfo reads the turn parameters from the room row.
func roomInfo(roomUUID string) (createdBy int64, timerSecs int, wrongLimit int, ok bool) {
	db, err := pgdb.GetInstance()
	if err != nil {
		return 0, 60, 3, false
	}
	row, err := db.GetOne(`SELECT created_by, timer, wrong_answers FROM prooms WHERE uuid = $1`, roomUUID)
	if err != nil || row == nil {
		return 0, 60, 3, false
	}
	createdBy = functions.Coerce[int64](row["created_by"])
	timerSecs = functions.Coerce[int](row["timer"])
	if timerSecs <= 0 {
		timerSecs = 60
	}
	wrongLimit = functions.Coerce[int](row["wrong_answers"])
	if wrongLimit <= 0 {
		wrongLimit = 3
	}
	return createdBy, timerSecs, wrongLimit, true
}

// assignWords deranges the players' words: each player wears a word submitted by
// another (rotation by one over the join order, so never their own).
func assignWords(st *RoomState) {
	n := len(st.Order)
	st.Assigned = map[string]string{}
	if n < 2 {
		return
	}
	for i, key := range st.Order {
		owner := st.Order[(i+1)%n]
		st.Assigned[key] = st.Players[owner].Word
	}
}

// expireTurn ends a running turn whose countdown has passed and advances to the
// next player (which then waits for the creator's Start).
func expireTurn(st *RoomState) {
	if st.Started && !st.Deadline.IsZero() && time.Now().After(st.Deadline) {
		st.Started = false
		st.Wrong = 0
		st.ActiveIx++
	}
}

// --- HTTP handlers --------------------------------------------------------

type joinBody struct {
	Name string `json:"name"`
	Word string `json:"word"`
}

// JoinGame sets/updates the caller's game name + word and adds them to the room
// (in join order = turn order). POST /papers/{roomId}/game/join
func JoinGame(w http.ResponseWriter, r *http.Request) {
	roomUUID := mux.Vars(r)["roomId"]
	key := playerKey(r)
	if key == "" {
		functions.JSONError(w, http.StatusUnauthorized, "no session")
		return
	}
	var body joinBody
	_ = json.NewDecoder(r.Body).Decode(&body)

	// Persist the game identity under the session key (pre-fills next time).
	SetGameUser(GameUser{SessionKey: key, Name: body.Name, Word: body.Word})

	st, _ := GetRoomState(roomUUID)
	if st.Players == nil {
		st.Players = map[string]GameUser{}
	}
	if _, seen := st.Players[key]; !seen {
		st.Order = append(st.Order, key) // first join => turn position
	}
	st.Players[key] = GameUser{SessionKey: key, Name: body.Name, Word: body.Word}
	SetRoomState(st)
	hub.notify(roomUUID)

	cb, _, _, _ := roomInfo(roomUUID)
	writeState(w, roomUUID, key, isRoomCreator(r, cb))
}

// GameState returns the caller's view of the room: everyone else's assigned word
// (never their own), turn info, and the countdown deadline.
// GET /papers/{roomId}/game/state
func GameState(w http.ResponseWriter, r *http.Request) {
	roomUUID := mux.Vars(r)["roomId"]
	cb, _, _, _ := roomInfo(roomUUID)
	writeState(w, roomUUID, playerKey(r), isRoomCreator(r, cb))
}

type turnBody struct {
	Action string `json:"action"` // start | wrong | end | restart
}

// TurnAction runs a creator-only turn command. POST /papers/{roomId}/game/turn
func TurnAction(w http.ResponseWriter, r *http.Request) {
	roomUUID := mux.Vars(r)["roomId"]
	createdBy, timerSecs, wrongLimit, ok := roomInfo(roomUUID)
	if !ok {
		functions.JSONError(w, http.StatusNotFound, "room not found")
		return
	}
	s := cache.SessionFromContext(r.Context())
	if s == nil || int64(s.UserID) != createdBy {
		functions.JSONError(w, http.StatusForbidden, "only the room creator can control the game")
		return
	}

	var body turnBody
	_ = json.NewDecoder(r.Body).Decode(&body)

	st, _ := GetRoomState(roomUUID)

	switch body.Action {
	case "start":
		// Begin (or resume for the next player) the active player's countdown.
		if len(st.Assigned) == 0 {
			assignWords(&st)
		}
		if st.ActiveIx >= len(st.Order) {
			st.ActiveIx = 0 // wrap
		}
		st.Started = true
		st.Wrong = 0
		st.Deadline = time.Now().Add(time.Duration(timerSecs) * time.Second)
		scheduleExpiry(roomUUID, timerSecs) // fire a ping when the timer runs out

	case "wrong":
		if st.Started {
			st.Wrong++
			if st.Wrong >= wrongLimit {
				// End the turn instantly and move to the next player.
				st.Started = false
				st.Wrong = 0
				st.Deadline = time.Now()
				st.ActiveIx++
			}
		}

	case "restart":
		// New round: clear words + assignments, players re-prompted to submit.
		for k, p := range st.Players {
			p.Word = ""
			st.Players[k] = p
		}
		st.Assigned = map[string]string{}
		st.ActiveIx = 0
		st.Wrong = 0
		st.Started = false
		st.Deadline = time.Time{}

	case "end":
		// Flag the room for deletion and drop the live state.
		if db, err := pgdb.GetInstance(); err == nil {
			_, _ = db.Exec(`UPDATE prooms SET deleted = true WHERE uuid = $1`, roomUUID)
		}
		RoomStateCache.Delete(roomUUID)
		functions.WriteJSON(w, http.StatusOK, map[string]any{"ended": true})
		return

	default:
		functions.JSONError(w, http.StatusBadRequest, "unknown action")
		return
	}

	SetRoomState(st)
	hub.notify(roomUUID)
	writeState(w, roomUUID, playerKey(r), true)
}

// writeState returns the room state as the given caller should see it.
func writeState(w http.ResponseWriter, roomUUID, selfKey string, isCreator bool) {
	createdBy, timerSecs, wrongLimit, _ := roomInfo(roomUUID)
	st, _ := GetRoomState(roomUUID)
	expireTurn(&st)
	SetRoomState(st)

	type playerView struct {
		Key    string `json:"key"`
		Name   string `json:"name"`
		Word   string `json:"word,omitempty"` // the word this player WEARS (blank for self)
		Ready  bool   `json:"ready"`
		Active bool   `json:"active"`
	}
	players := make([]playerView, 0, len(st.Order))
	for i, key := range st.Order {
		p := st.Players[key]
		pv := playerView{
			Key:    key,
			Name:   p.Name,
			Ready:  p.Word != "",
			Active: st.Started && i == st.ActiveIx,
		}
		// Everyone sees others' assigned words, never their own.
		if key != selfKey {
			pv.Word = st.Assigned[key]
		}
		players = append(players, pv)
	}

	activeKey := ""
	if st.ActiveIx >= 0 && st.ActiveIx < len(st.Order) {
		activeKey = st.Order[st.ActiveIx]
	}

	remaining := 0
	if st.Started && !st.Deadline.IsZero() {
		if d := time.Until(st.Deadline); d > 0 {
			remaining = int(d.Seconds())
		}
	}

	self, _ := GetGameUser(selfKey)
	functions.WriteJSON(w, http.StatusOK, map[string]any{
		"selfName":   self.Name,
		"selfWord":   self.Word,
		"isCreator":  isCreator,
		"players":    players,
		"activeKey":  activeKey,
		"started":    st.Started,
		"wrong":      st.Wrong,
		"wrongLimit": wrongLimit,
		"timerSecs":  timerSecs,
		"remaining":  remaining,
		"deadline":   st.Deadline.Unix(),
		"createdBy":  createdBy,
		"selfKey":    selfKey,
	})
}

// GameEvents is the Server-Sent Events stream for a room. It emits a lightweight
// "changed" event whenever the room's state changes; the client then pulls its
// own /game/state. GET /papers/{roomId}/game/events
func GameEvents(w http.ResponseWriter, r *http.Request) {
	room := mux.Vars(r)["roomId"]
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch := hub.subscribe(room)
	defer hub.unsubscribe(room, ch)

	fmt.Fprint(w, "event: ready\ndata: {}\n\n")
	flusher.Flush()

	keep := time.NewTicker(25 * time.Second)
	defer keep.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ch:
			fmt.Fprint(w, "data: changed\n\n")
			flusher.Flush()
		case <-keep.C:
			fmt.Fprint(w, ": ping\n\n") // comment line keeps the connection alive
			flusher.Flush()
		}
	}
}
