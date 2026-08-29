package words

import (
	. "tls-rest/go/engine/controllers/field"
	. "tls-rest/go/engine/controllers/module"
)

// WordsGroupID is the user group allowed to use this module. The module denies
// everyone by default (DefaultPermission below); grant this group its modes in
// user_group_rights so ONLY its members get access (item 4). See NewWords.
const WordsGroupID = 2 // <-- set to your app's "learners" group id

// fieldset defines the module's fields.
//
//   - word:    stored, user-entered.
//   - tries/success/fail: stored counters, maintained only by the
//     registerResult route (read-only + not submitted from forms).
//   - translations: CALCULATED, not stored — a virtual field computed in SQL.
//   - win_rate:     CALCULATED, not stored — success / (success + fail).
func (m *Words) fieldset() []Field {
	return []Field{
		NewField("word", TYPE_STRING, true).
			WithLabel("Word").
			WithDescription("The word to practise").
			WithValidation("minLength", 1).
			WithValidation("maxLength", 200),

		// Calculated, not stored. AsVirtual keeps it out of INSERT/UPDATE; the
		// value is produced by the WithSQL expression at read time. The default
		// returns an empty JSON array so the module runs out of the box — replace
		// the expression with your translation source, e.g.:
		//   (SELECT COALESCE(json_agg(t.text ORDER BY t.text), '[]'::json)
		//      FROM word_translations t WHERE t.word_id = words.id)
		NewField("translations", TYPE_JSON, false).
			WithLabel("Translations").
			WithDescription("Known translations (computed)").
			AsVirtual().
			AsReadOnly().
			NonSortable().
			NonSearchable().
			WithSQL("'[]'::json"),

		NewField("tries", TYPE_INT, false).
			WithLabel("Tries").
			WithDefault(0).
			AsReadOnly().
			NotSubmitted(),

		NewField("success", TYPE_INT, false).
			WithLabel("Success").
			WithDefault(0).
			AsReadOnly().
			NotSubmitted(),

		NewField("fail", TYPE_INT, false).
			WithLabel("Fail").
			WithDefault(0).
			AsReadOnly().
			NotSubmitted(),

		// Calculated, not stored: success rate in [0,1], 0 when never attempted.
		NewField("win_rate", TYPE_FLOAT, false).
			WithLabel("Win rate").
			WithDescription("success / (success + fail)").
			AsVirtual().
			AsReadOnly().
			NonSearchable().
			WithSQL("CASE WHEN (COALESCE(success,0) + COALESCE(fail,0)) > 0 THEN round(COALESCE(success,0)::numeric / (COALESCE(success,0) + COALESCE(fail,0)), 4) ELSE 0 END"),
	}
}

// NewWords builds the module instance.
func NewWords() *Words {
	m := &Words{
		ModuleAbstract: &ModuleAbstract[interface{}]{
			ID:   "words",
			Name: "Words",

			// Item 1 — per-user data: a user only ever lists their own words.
			OwnerScoped: true,
			Submenu:     "External",

			// Item 4 — access restricted to a specific user group. Deny by
			// default, then grant WordsGroupID in the DB (the table resolve.go
			// reads). 31 = list|view|create|edit|delete (create is required by the
			// register route):
			//
			//   INSERT INTO user_group_rights (group_id, module, modes)
			//   VALUES (2, 'words', 31)
			//   ON CONFLICT (group_id, module) DO UPDATE SET modes = EXCLUDED.modes;
			//
			// (or grant it via the module-rights admin UI). No other group can see
			// or use the module.
			DefaultPermission:    0, // PERMISSION_DENY
			DefaultPermissionSet: true,
			Rights:               make(map[int]int),
		},
	}

	m.ModuleAbstract.Fields = m.fieldset()
	m.ModuleAbstract.Filters = m.filters()

	// Item 3 — custom action route: POST /words/{id}/register {result}.
	m.ModuleAbstract.CustomRoutes = []CustomRoute{
		{Path: "/{id}/register", Methods: []string{"POST"}, Handler: m.registerResult},
	}

	m.Initialize("words")
	return m
}

// Module is the global instance (set by Init at startup).
var Module *Words

func Init() { Module = NewWords() }
