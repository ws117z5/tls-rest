package jobs

import (
	"time"

	actionsctl "tls-rest/go/engine/controllers/actions"
	"tls-rest/go/engine/controllers/turn"
)

const usageCheckInterval = 10 * time.Minute

func init() {
	a := &actionsctl.Action{
		ID:          "turn_usage",
		Name:        "TURN usage check",
		Description: "Polls Cloudflare's TURN analytics and suspends new TURN credentials once the monthly cap is hit.",
		Run:         turn.CheckUsage,
	}
	actionsctl.Register(a)
	a.SetSchedule(usageCheckInterval)
	go func() { _, _ = a.RunNow() }() // populate the cache once at startup rather than waiting a full interval
}
