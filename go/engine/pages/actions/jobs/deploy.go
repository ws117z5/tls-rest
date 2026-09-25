package jobs

import actionsctl "tls-rest/go/engine/controllers/actions"

func init() {
	actionsctl.Register(&actionsctl.Action{
		ID:          "deploy",
		Name:        "Deploy (init/deploy.sh)",
		Description: "Runs the deploy script in a live terminal: schema diff, rights/config data sync and file deploys; answer its prompts and type the ssh password below.",
		Command:     []string{"bash", "deploy.sh"},
		Dir:         "init",
	})
}
