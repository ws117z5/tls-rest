package papers

import (
	"net/http"

	. "tls-rest/go/engine/controllers/field"
	. "tls-rest/go/engine/controllers/module"
)

// Papers as a MODULE: list = active games (rooms), view = one room (rendered by
// the frontend modules/papers/view.tsx video grid). Rooms are soft-deleted
// (flagged). The module keys on a stored `hash` column rather than the real
// uuid — see the "hash" field below and roomhash.go — so the internal uuid
// never appears in a URL or API response. The WebRTC mesh signaling stays as
// absolute custom routes.
type PapersModule struct {
	*ModuleAbstract[interface{}]
}

// setRoomHash is an AfterFieldset hook: on create, the engine has just
// generated the row's uuid (see BaseController.Create) — derive and store its
// hash right here, once, so every lookup afterwards is a plain indexed
// `WHERE hash = $1` instead of hashing every row on every request. A no-op on
// edit (uuid isn't in the submitted data there), which correctly leaves the
// stored hash untouched.
func setRoomHash(_ *http.Request, data map[string]interface{}) (map[string]interface{}, error) {
	if u, ok := data["uuid"].(string); ok && u != "" {
		data["hash"] = hashRoomUUID(u)
	}
	return data, nil
}

func (m *PapersModule) fieldset() []Field {
	return []Field{
		// The public room identifier (KeyField below): a MurmurHash3 of the real
		// uuid, computed once at creation by the setRoomHash hook and stored —
		// see roomhash.go. Auto-generated and read-only, so it never appears in
		// create (nothing to show yet) or edit (nothing to change) — list/view
		// only, same as the id/uuid system fields.
		NewField("hash", TYPE_STRING, false).
			WithLabel("Room Code").
			AsReadOnly().
			InModes(MODE_LIST | MODE_VIEW),

		NewField("name", TYPE_STRING, true).
			WithLabel("Game").
			WithValidation("minLength", 1),

		// Password is never returned to any client (TYPE_PASSWORD fields are
		// write-only at the engine level — see accessfilter.fieldReadableInData),
		// so listing/viewing rooms can't leak it. Whether a room is protected is
		// exposed via has_password below.
		NewField("password", TYPE_PASSWORD, false).
			WithLabel("Password"),

		// Virtual: true when the room is password-protected. Drives the join prompt.
		NewField("has_password", TYPE_CHECKBOX, false).
			WithLabel("Protected").
			AsVirtual().
			AsReadOnly().
			AsAdminOnly().
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
			Icon:       "",
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
	// auto /papers/{uuid} view so the literal segments (report/plan/create) win.
	m.ModuleAbstract.CustomRoutes = []CustomRoute{
		{Path: "/papers/create", Methods: []string{http.MethodPost}, Handler: CreateRoom, Absolute: true},
		{Path: "/papers/{roomId}/game/join", Methods: []string{http.MethodPost}, Handler: JoinGame, Absolute: true},
		{Path: "/papers/{roomId}/game/state", Methods: []string{http.MethodGet}, Handler: GameState, Absolute: true},
		{Path: "/papers/{roomId}/game/events", Methods: []string{http.MethodGet}, Handler: GameEvents, Absolute: true},
		{Path: "/papers/{roomId}/game/turn", Methods: []string{http.MethodPost}, Handler: TurnAction, Absolute: true},
		{Path: "/papers/{roomId}/game/signal", Methods: []string{http.MethodPost}, Handler: SendSignal, Absolute: true},
		{Path: "/papers/{roomId}/game/signal", Methods: []string{http.MethodGet}, Handler: DrainSignals, Absolute: true},
		{Path: "/papers/{roomId}/report", Methods: []string{http.MethodPost}, Handler: ReportLink, Absolute: true},
		{Path: "/papers/{roomId}/plan", Methods: []string{http.MethodGet}, Handler: GetPlan, Absolute: true},
		{Path: "/papers/{roomId}/{userId}", Methods: []string{http.MethodPost}, Handler: RegisterUser, Absolute: true},
	}
	return m
}

// InitModule registers papers as a module over the `papers` table. Use this
// instead of the old page Init().
func Init() {
	neg = NewNegotiator() // was initialised by the old page Init()
	NewPapersModule().Initialize("papers")
}
