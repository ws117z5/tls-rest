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

// Docker-only path; local dev overrides via RIG_BINARY.
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

// The rig provisions its own test accounts per scenario; admin_uuid/server
// are injected by actions.go's fillRequestDefaults, not supplied by the admin.
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
		// Forwarded explicitly: .env-resolved values aren't real OS env vars (see actions.Action.Env).
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
