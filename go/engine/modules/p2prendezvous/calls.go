package p2prendezvous

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"

	"tls-rest/go/engine/controllers/functions"
	"tls-rest/go/engine/controllers/mesh"
	"tls-rest/go/engine/controllers/module"
	"tls-rest/go/engine/controllers/relaynode"
	"tls-rest/go/engine/modules/chats"
)

// callChatPrefix namespaces call signaling/planning under its own room key,
// separate from signalChatPrefix, so a call's offer/answer/ICE traffic never
// mixes with ordinary 1:1 message-transport signaling in the same queue.
// Reuses the same mesh.Coordinator (go/engine/controllers/mesh) already
// backing papers' group video rooms.
const callChatPrefix = "call:"

func writeCallPlan(w http.ResponseWriter, env *mesh.PlanEnvelope, waiting []string) {
	if env == nil {
		functions.WriteJSON(w, http.StatusOK, map[string]interface{}{"status": "waiting", "waiting": waiting})
		return
	}
	functions.WriteJSON(w, http.StatusOK, env)
}

// handleCallReport records the caller's own measured link capacities for
// chat {id}'s call and returns the resulting relay/video-tier plan. Platform
// is always "android" here (unlike papers' mesh.ReportLink, which trusts a
// client-asserted peer/platform) — this handler is p2pmsg-android-only.
func handleCallReport(w http.ResponseWriter, r *http.Request) {
	s := requireSession(w, r)
	if s == nil {
		return
	}
	chatID, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil || chatID <= 0 {
		functions.JSONError(w, http.StatusBadRequest, "bad chat")
		return
	}
	ok, err := chats.IsAcceptedMember(r.Context(), chatID, s.UserID)
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "query failed")
		return
	}
	if !ok {
		functions.JSONError(w, http.StatusForbidden, "not a member of that chat")
		return
	}

	var body struct {
		Up    float64         `json:"up"`
		Down  float64         `json:"down"`
		Stats []mesh.LinkStat `json:"stats"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		functions.JSONError(w, http.StatusBadRequest, "bad report")
		return
	}

	room := callChatPrefix + strconv.FormatInt(chatID, 10)
	self := strconv.Itoa(s.UserID)
	mesh.DefaultCoordinator().Report(room, self, &mesh.Report{
		Up: body.Up, Down: body.Down, Stats: body.Stats, Platform: "android",
	})
	env, waiting := mesh.DefaultCoordinator().Plan(room)
	writeCallPlan(w, env, waiting)
}

// handleCallPlan returns the current plan for chat {id}'s call without
// reporting new measurements — for the polling fallback (see CallProber.kt).
func handleCallPlan(w http.ResponseWriter, r *http.Request) {
	s := requireSession(w, r)
	if s == nil {
		return
	}
	chatID, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil || chatID <= 0 {
		functions.JSONError(w, http.StatusBadRequest, "bad chat")
		return
	}
	ok, err := chats.IsAcceptedMember(r.Context(), chatID, s.UserID)
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "query failed")
		return
	}
	if !ok {
		functions.JSONError(w, http.StatusForbidden, "not a member of that chat")
		return
	}
	env, waiting := mesh.DefaultCoordinator().Plan(callChatPrefix + strconv.FormatInt(chatID, 10))
	writeCallPlan(w, env, waiting)
}

// handleCallSignal relays one opaque WebRTC signaling message (offer/answer/
// candidate) to one other call participant — same auth shape as handleSignal.
func handleCallSignal(w http.ResponseWriter, r *http.Request) {
	s := requireSession(w, r)
	if s == nil {
		return
	}
	chatID, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil || chatID <= 0 {
		functions.JSONError(w, http.StatusBadRequest, "bad chat")
		return
	}

	var body struct {
		To   int             `json:"to"` // 0 == the server's own relay peer (see relaynode)
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.To < 0 || len(body.Data) == 0 {
		functions.JSONError(w, http.StatusBadRequest, "bad signal")
		return
	}

	ok, err := chats.IsAcceptedMember(r.Context(), chatID, s.UserID)
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "query failed")
		return
	}
	if !ok {
		functions.JSONError(w, http.StatusForbidden, "not a member of that chat")
		return
	}

	room := callChatPrefix + strconv.FormatInt(chatID, 10)

	// The server (peer id "0") is implicitly "in" every room for signaling
	// purposes, not a real chat member, so it skips the membership check a
	// human target still needs.
	if body.To == 0 {
		if err := relaynode.DefaultManager().HandleSignal(room, strconv.Itoa(s.UserID), body.Data); err != nil {
			functions.JSONError(w, http.StatusInternalServerError, "relay signal failed")
			return
		}
		functions.WriteJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
		return
	}

	memberIDs, err := chats.AcceptedMemberIDs(r.Context(), chatID, s.UserID)
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "query failed")
		return
	}
	targetIsMember := false
	for _, id := range memberIDs {
		if id == body.To {
			targetIsMember = true
			break
		}
	}
	if !targetIsMember {
		functions.JSONError(w, http.StatusForbidden, "target not a member of that chat")
		return
	}

	mesh.QueueSignal(room, strconv.Itoa(body.To), strconv.Itoa(s.UserID), body.Data)
	hub.notify(body.To)

	functions.WriteJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

// handleCallInvite rings every other accepted member of chat {id} — unlike
// handleCallSignal's single `to`, this fans out to the whole chat.
func handleCallInvite(w http.ResponseWriter, r *http.Request) {
	s := requireSession(w, r)
	if s == nil {
		return
	}
	chatID, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil || chatID <= 0 {
		functions.JSONError(w, http.StatusBadRequest, "bad chat")
		return
	}

	var body struct {
		Media string `json:"media"` // "audio" | "video"
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || (body.Media != "audio" && body.Media != "video") {
		functions.JSONError(w, http.StatusBadRequest, "bad invite")
		return
	}

	ok, err := chats.IsAcceptedMember(r.Context(), chatID, s.UserID)
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "query failed")
		return
	}
	if !ok {
		functions.JSONError(w, http.StatusForbidden, "not a member of that chat")
		return
	}

	memberIDs, err := chats.AcceptedMemberIDs(r.Context(), chatID, s.UserID)
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "query failed")
		return
	}

	payload, err := json.Marshal(map[string]interface{}{"type": "invite", "chatId": chatID, "media": body.Media})
	if err != nil {
		functions.JSONError(w, http.StatusInternalServerError, "encode failed")
		return
	}
	room := callChatPrefix + strconv.FormatInt(chatID, 10)
	self := strconv.Itoa(s.UserID)
	for _, memberID := range memberIDs {
		if memberID == s.UserID {
			continue
		}
		mesh.QueueSignal(room, strconv.Itoa(memberID), self, payload)
		hub.notify(memberID)
	}

	functions.WriteJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

func init() {
	module.AddRouteRegistrar(func(router *mux.Router) {
		router.HandleFunc("/api/p2p/chats/{id}/call/report", handleCallReport).Methods("POST")
		router.HandleFunc("/api/p2p/chats/{id}/call/plan", handleCallPlan).Methods("GET")
		router.HandleFunc("/api/p2p/chats/{id}/call/signal", handleCallSignal).Methods("POST")
		router.HandleFunc("/api/p2p/chats/{id}/call/invite", handleCallInvite).Methods("POST")
	})

	// Wakes a call participant's SSE stream when the server's own relay peer
	// (relaynode) queues them a signal in-process — the human-to-human path
	// above already calls hub.notify itself right after QueueSignal, so this
	// only matters for signals the server originates. Filtered to this
	// module's own room prefix since mesh.QueueSignal's room/`to` keyspace is
	// shared with other modules (e.g. papers) whose `to` is a non-numeric
	// player key.
	mesh.RegisterSignalQueuedHook(func(room, to string) {
		if !strings.HasPrefix(room, callChatPrefix) {
			return
		}
		if toID, err := strconv.Atoi(to); err == nil {
			hub.notify(toID)
		}
	})
}
