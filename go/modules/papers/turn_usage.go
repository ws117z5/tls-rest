// Cloudflare's Realtime TURN has no built-in usage cap or kill switch, so this
// polls its GraphQL Analytics API (the byte counters are authoritative —
// there's no client-side self-reporting to trust or that could be skipped on
// a bad disconnect) for the current calendar month's egress+ingress bytes and
// stops GetIceServers from handing out TURN credentials once
// config.TurnMonthlyCapMB is exceeded. Registered as a scheduled Action so it
// runs on its own, and is also visible/runnable from the Actions admin page.
package papers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	config "tls-rest/go/constants"
	"tls-rest/go/engine/controllers/actions"
)

var turnUsageCheckInterval = 10 * time.Minute

// turnUsage is the last-checked cap state, read by GetIceServers on every
// request and written only by checkTurnUsage.
var turnUsage struct {
	mu        sync.RWMutex
	overCap   bool
	usedBytes int64
	checkedAt time.Time
}

// turnCapExceeded reports whether the last usage check found the configured
// monthly cap exceeded. Always false when TurnMonthlyCapMB is unset (0) or
// before the first successful check has ever run.
func turnCapExceeded() bool {
	turnUsage.mu.RLock()
	defer turnUsage.mu.RUnlock()
	return turnUsage.overCap
}

type cfGraphQLResponse struct {
	Data struct {
		Viewer struct {
			Accounts []struct {
				// One row per hour (query groups by datetimeHour, the coarsest
				// dimension Cloudflare offers) — bounded to <=744 rows for a
				// 31-day month, well under the query's own limit.
				Groups []struct {
					Sum struct {
						EgressBytes  int64 `json:"egressBytes"`
						IngressBytes int64 `json:"ingressBytes"`
					} `json:"sum"`
				} `json:"callsTurnUsageAdaptiveGroups"`
			} `json:"accounts"`
		} `json:"viewer"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// checkTurnUsage queries Cloudflare's TURN analytics for bytes used since the
// start of the current UTC month and updates the cached cap state. Registered
// as the "papers_turn_usage" action.
func checkTurnUsage() (string, error) {
	if config.TurnMonthlyCapMB <= 0 || config.CFAccountID == "" {
		return "cap check disabled (TURN_MONTHLY_CAP_MB or CF_ACCOUNT_ID not set)", nil
	}
	token := config.CFAnalyticsToken
	if token == "" {
		token = config.CFTurnToken
	}
	if token == "" {
		return "cap check disabled (no analytics token)", nil
	}

	now := time.Now().UTC()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	const dateFmt = "2006-01-02"

	const rowLimit = 1000 // >= 744 (31 days x 24h), the max possible datetimeHour rows in a month
	query := `query($accountId: String!, $from: Date!, $to: Date!, $keyId: String) {
		viewer {
			accounts(filter: { accountTag: $accountId }) {
				callsTurnUsageAdaptiveGroups(limit: ` + fmt.Sprint(rowLimit) + `, filter: { date_geq: $from, date_leq: $to, keyId: $keyId }) {
					dimensions { datetimeHour }
					sum { egressBytes ingressBytes }
				}
			}
		}
	}`
	payload, err := json.Marshal(map[string]any{
		"query": query,
		"variables": map[string]any{
			"accountId": config.CFAccountID,
			"from":      monthStart.Format(dateFmt),
			"to":        now.Format(dateFmt),
			// Scopes usage to this one TURN app — without it, the sum would
			// include every TURN key on the whole Cloudflare account.
			"keyId": config.CFTurnKeyID,
		},
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, "https://api.cloudflare.com/client/v4/graphql", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var cf cfGraphQLResponse
	if err := json.NewDecoder(resp.Body).Decode(&cf); err != nil {
		return "", err
	}
	if len(cf.Errors) > 0 {
		return "", fmt.Errorf("cloudflare graphql: %s", cf.Errors[0].Message)
	}

	// Sum across every returned row rather than assuming a single aggregate
	// row — correct regardless of how many hourly buckets came back.
	var totalBytes int64
	var rowCount int
	for _, acc := range cf.Data.Viewer.Accounts {
		rowCount += len(acc.Groups)
		for _, g := range acc.Groups {
			totalBytes += g.Sum.EgressBytes + g.Sum.IngressBytes
		}
	}

	capBytes := int64(config.TurnMonthlyCapMB) * 1024 * 1024
	over := totalBytes >= capBytes

	// rowCount hitting the query's own limit means some hours may be missing
	// from totalBytes — an undercount. That's still safe to act on when it
	// already shows over-cap (the real total is at least this high), but an
	// undercount showing under-cap is not trustworthy: the real total could
	// be higher. Bail out (keeping whatever state was cached before) rather
	// than risk reporting "under cap" when it might not be.
	if rowCount >= rowLimit && !over {
		return "", fmt.Errorf("turn usage query hit its row limit (%d) with an under-cap result — "+
			"treating as unknown rather than risking a false all-clear", rowLimit)
	}

	turnUsage.mu.Lock()
	turnUsage.usedBytes = totalBytes
	turnUsage.overCap = over
	turnUsage.checkedAt = time.Now()
	turnUsage.mu.Unlock()

	usedMB := totalBytes / (1024 * 1024)
	if over {
		return fmt.Sprintf("%d MB used of %d MB cap — TURN credentials suspended for the rest of the month",
			usedMB, config.TurnMonthlyCapMB), nil
	}
	return fmt.Sprintf("%d MB used of %d MB cap", usedMB, config.TurnMonthlyCapMB), nil
}

func initTurnUsageAction() {
	a := &actions.Action{
		ID:          "papers_turn_usage",
		Name:        "Papers TURN usage check",
		Description: "Polls Cloudflare's TURN analytics and suspends new TURN credentials once the monthly cap is hit.",
		Run:         checkTurnUsage,
	}
	actions.Register(a)
	a.SetSchedule(turnUsageCheckInterval)
	go func() { _, _ = a.RunNow() }() // populate the cache once at startup rather than waiting a full interval
}
