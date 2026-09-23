package papers

import (
	"fmt"
	"net/http"

	"tls-rest/go/app"
	. "tls-rest/go/engine/controllers/field"
	"tls-rest/go/engine/controllers/mesh"
	. "tls-rest/go/engine/controllers/module"

	"github.com/ws117z5/mmh3"
)

// PapersModule backs the papers game module (list = rooms, view = one room).
type PapersModule struct {
	*ModuleAbstract[interface{}]
}

const roomHashSeed uint32 = 0x50415052 // "PAPR"

// hashRoomUUID derives a room's public identifier from its real uuid via
// MurmurHash3 (32-bit)
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

// setRoomHash is an AfterFieldset hook: derives and stores the hash on create,
// a no-op on edit.
func setRoomHash(_ *http.Request, data map[string]interface{}) (map[string]interface{}, error) {
	if u, ok := data["uuid"].(string); ok && u != "" {
		data["hash"] = hashRoomUUID(u)
	}
	return data, nil
}

func (m *PapersModule) fieldset() []Field {
	return []Field{
		// KeyField below: a MurmurHash3 of the real uuid, set by setRoomHash.
		NewField("hash", TYPE_STRING, false).
			WithLabel("Room Code").
			AsReadOnly().
			InModes(MODE_LIST | MODE_VIEW),

		NewField("name", TYPE_STRING, true).
			WithLabel("Game").
			WithValidation("minLength", 1),

		// TYPE_PASSWORD is write-only at the engine level; has_password below exposes whether it's set.
		NewField("password", TYPE_PASSWORD, false).
			WithLabel("Password"),

		// Virtual: true when the room is password-protected. Drives the join prompt.
		NewField("has_password", TYPE_CHECKBOX, false).
			WithLabel("Protected").
			AsVirtual().
			AsReadOnly().
			WithSQL("(password IS NOT NULL AND password <> '')"),

		// Round countdown as a single duration (stored as total seconds).
		NewField("timer", TYPE_TIME_DURATION, false).
			WithLabel("Round timer").
			WithDefault(60).
			WithExtraParams(map[string]interface{}{"format": "mm:ss"}),

		// How many "wrong" clicks end a player's turn instantly.
		NewField("wrong_answers", TYPE_INT, false).
			WithLabel("Wrong answers to skip").
			WithDefault(3),

		// Player list (JSON), maintained by the signaling handlers — read-only here.
		NewField("users", TYPE_JSON, false).
			WithLabel("Players").
			AsReadOnly().
			AsAdminOnly().
			NonSortable(),

		// Soft-delete flag; hidden from forms, drives SoftDelete filtering.
		NewField("deleted", TYPE_CHECKBOX, false).
			WithLabel("Deleted").
			AsReadOnly().
			AsAdminOnly().
			WithDefault(false),
	}
}

func NewPapersModule() *PapersModule {
	m := &PapersModule{
		ModuleAbstract: &ModuleAbstract[interface{}]{
			ID:         "papers",
			Name:       "Papers",
			Icon:       "papers",
			Submenu:    "games",
			KeyField:   "hash", // rooms are addressed by their stored hash, not uuid or id
			SoftDelete: true,   // DELETE flags `deleted`; deleted rows are hidden
			// Every authorized user can list/view/create games.
			DefaultPermission:    PERMISSION_WRITE,
			DefaultPermissionSet: true,
		},
	}
	m.ModuleAbstract.Fields = m.fieldset()
	m.ModuleAbstract.AfterFieldset = setRoomHash

	// WebRTC mesh signaling — absolute paths, registered before the module's
	// auto /papers/{uuid} view so the literal segments (report/plan) win.
	m.ModuleAbstract.CustomRoutes = []CustomRoute{
		{Path: "/papers/{roomId}/game/join", Methods: []string{http.MethodPost}, Handler: JoinGame, Absolute: true},
		{Path: "/papers/{roomId}/game/state", Methods: []string{http.MethodGet}, Handler: GameState, Absolute: true},
		{Path: "/papers/{roomId}/game/events", Methods: []string{http.MethodGet}, Handler: GameEvents, Absolute: true},
		{Path: "/papers/{roomId}/game/turn", Methods: []string{http.MethodPost}, Handler: TurnAction, Absolute: true},
		{Path: "/papers/{roomId}/game/signal", Methods: []string{http.MethodPost}, Handler: SendSignal, Absolute: true},
		{Path: "/papers/{roomId}/game/signal", Methods: []string{http.MethodGet}, Handler: DrainSignals, Absolute: true},
		{Path: "/papers/{roomId}/report", Methods: []string{http.MethodPost}, Handler: mesh.ReportLink, Absolute: true},
		{Path: "/papers/{roomId}/plan", Methods: []string{http.MethodGet}, Handler: mesh.GetPlan, Absolute: true},
	}
	return m
}

func init() {
	app.RegisterModule(NewPapersModule(), "papers")
}
