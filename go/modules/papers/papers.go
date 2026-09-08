package papers

import (
	"net/http"

	. "tls-rest/go/engine/controllers/field"
	. "tls-rest/go/engine/controllers/module"
)

// Papers as a MODULE: list = active games (rooms), view = one room (rendered by
// the frontend modules/papers/view.tsx video grid). Rooms are addressed by uuid
// and soft-deleted (flagged), so the module keys on `uuid` and hides deleted
// rows. The WebRTC mesh signaling stays as absolute custom routes.
type PapersModule struct {
	*ModuleAbstract[interface{}]
}

func (m *PapersModule) fieldset() []Field {
	return []Field{
		NewField("name", TYPE_STRING, true).
			WithLabel("Game").
			WithValidation("minLength", 1),

		// Password is never returned to clients (admin-only) so listing rooms can't
		// leak it. Whether a room is protected is exposed via has_password below.
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
			Submenu:    "games",
			KeyField:   "uuid", // rooms are addressed by uuid, not incrementing id
			SoftDelete: true,   // DELETE flags `deleted`; deleted rows are hidden
			// Every authorized user can list/view/create games.
			DefaultPermission:    PERMISSION_WRITE,
			DefaultPermissionSet: true,
		},
	}
	m.ModuleAbstract.Fields = m.fieldset()

	// WebRTC mesh signaling — absolute paths, registered before the module's
	// auto /papers/{uuid} view so the literal segments (report/plan/create) win.
	m.ModuleAbstract.CustomRoutes = []CustomRoute{
		{Path: "/papers/create", Methods: []string{http.MethodPost}, Handler: CreateRoom, Absolute: true},
		{Path: "/papers/{roomId}/game/join", Methods: []string{http.MethodPost}, Handler: JoinGame, Absolute: true},
		{Path: "/papers/{roomId}/game/state", Methods: []string{http.MethodGet}, Handler: GameState, Absolute: true},
		{Path: "/papers/{roomId}/game/events", Methods: []string{http.MethodGet}, Handler: GameEvents, Absolute: true},
		{Path: "/papers/{roomId}/game/turn", Methods: []string{http.MethodPost}, Handler: TurnAction, Absolute: true},
		{Path: "/papers/{roomId}/report", Methods: []string{http.MethodPost}, Handler: ReportLink, Absolute: true},
		{Path: "/papers/{roomId}/plan", Methods: []string{http.MethodGet}, Handler: GetPlan, Absolute: true},
		{Path: "/papers/{roomId}/{userId}", Methods: []string{http.MethodPost}, Handler: RegisterUser, Absolute: true},
	}
	return m
}

// InitModule registers papers as a module over the `prooms` table. Use this
// instead of the old page Init().
func Init() {
	neg = NewNegotiator() // was initialised by the old page Init()
	NewPapersModule().Initialize("prooms")
}
