package jobs

import (
	"fmt"
	"os"
	"strings"

	"tls-rest/go/app/constants"
	actionsctl "tls-rest/go/engine/controllers/actions"
	"tls-rest/go/engine/controllers/field"
	"tls-rest/go/engine/controllers/functions"
)

// defaultRigBinary is where the Dockerfile's "rig" stage bakes the compiled
// binary into the runtime image (rust-callmesh-rig, a separate repo — see
// that Dockerfile stage's comment) — only meaningful inside that container.
// Running the Go server directly on a dev machine (no /app there at all)
// needs RIG_BINARY pointed at a locally built one instead, e.g.:
//
//	export RIG_BINARY=/path/to/rust-callmesh-rig/rig/target/release/rig
//	(cd rust-callmesh-rig/rig && cargo build --release)  # builds that binary
const defaultRigBinary = "/app/bin/rig"

func rigBinaryPath() string {
	if p := strings.TrimSpace(os.Getenv("RIG_BINARY")); p != "" {
		return p
	}
	return defaultRigBinary
}

func rigScenarioOptions() []map[string]interface{} {
	return []map[string]interface{}{
		{"value": "user_call", "name": "User-to-user video call"},
		{"value": "group_call", "name": "Group call (mesh test)"},
		{"value": "multi_message", "name": "Multiple senders, different message types"},
		{"value": "cleanup", "name": "Cleanup — delete rig-created test users"},
	}
}

// The rig provisions whatever test accounts a scenario needs on its own
// (creates them, friends them with whoever is running the test, mints their
// session tokens) — see go/engine/pages/actions/actions.go's fillRequestDefaults
// for admin_uuid/server, which the admin never has to supply themselves. Every
// scenario-specific knob (group size, message count, which users to delete, …)
// goes in paramsJSON instead of its own field, since the knobs differ per
// scenario and the rig is what actually interprets them.
func init() {
	a := &actionsctl.Action{
		ID:          "test_rig",
		Name:        "Test rig",
		Description: "Runs rust-callmesh-rig against this backend: provisions its own test accounts, friends them with you, and exercises a real call or message flow.",
		Params: []field.Field{
			field.NewField("scenario", field.TYPE_SELECT, true).
				WithLabel("Scenario").WithDefault("user_call").WithOptions(rigScenarioOptions),
			field.NewField("params_json", field.TYPE_TEXT, false).
				WithLabel("Params (JSON)").
				WithDescription(`Scenario-specific overrides, e.g. {"group_size": 8} or {"cleanup_emails": ["..."]}. Leave blank for defaults.`),
		},
		CommandFunc: buildRigCommand,
		// The rig connects to Postgres directly (provisioning test users —
		// see rust-callmesh-rig's db.rs) using these same names, but a
		// subprocess only inherits *real* OS environment variables — these
		// are resolved from .env via constants.Env/RequireEnv (os.Setenv is
		// never called), so they don't exist as real env vars to inherit
		// unless forwarded explicitly here (see actions.Action.Env's doc).
		Env: []string{
			"PG_ADDR=" + constants.PDb.Addr,
			"PG_USER=" + constants.PDb.User,
			"PG_PASSWORD=" + constants.PDb.Password,
			fmt.Sprintf("USERS_GROUP_ID=%d", constants.UsersGroupID),
		},
	}
	actionsctl.Register(a)
}

func buildRigCommand(params map[string]interface{}) []string {
	str := func(name string) string { return strings.TrimSpace(functions.Coerce[string](params[name])) }

	paramsJSON := str("params_json")
	if paramsJSON == "" {
		paramsJSON = "{}"
	}

	argv := []string{
		rigBinaryPath(),
		"--mode", "scenario",
		"--scenario", str("scenario"),
		"--params", paramsJSON,
	}
	if adminUUID := str("admin_uuid"); adminUUID != "" {
		argv = append(argv, "--admin-uuid", adminUUID)
	}
	if server := str("server"); server != "" {
		argv = append(argv, "--server", server)
	}
	return argv
}
