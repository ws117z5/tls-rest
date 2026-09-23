// Cloudflare TURN usage-cap check, backing the "turn_usage" scheduled Action.
package turn

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	config "tls-rest/go/app/constants"
	"tls-rest/go/engine/controllers/actions"
)

var usageCheckInterval = 10 * time.Minute

// usage is the last-checked cap state, read by GetIceServers, written by checkUsage.
var usage struct {
	mu        sync.RWMutex
	overCap   bool
	usedBytes int64
	checkedAt time.Time
}

// CapExceeded reports whether the last usage check found the configured
// monthly cap exceeded. Always false when TurnMonthlyCapMB is unset (0) or
// before the first successful check has ever run.
func CapExceeded() bool {
	usage.mu.RLock()
	defer usage.mu.RUnlock()
	return usage.overCap
}

type cfGraphQLResponse struct {
	Data struct {
		Viewer struct {
			Accounts []struct {
				// One row per hour, <=744 for a 31-day month.
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

// checkUsage queries Cloudflare's TURN analytics for bytes used since the
// start of the current UTC month and updates the cached cap state. Registered
// as the "turn_usage" action.
func checkUsage() (string, error) {
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
			"keyId":     config.CFTurnKeyID, // scopes usage to this one TURN app
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

	// Hitting the row limit under-cap is untrustworthy (real total could be higher).
	if rowCount >= rowLimit && !over {
		return "", fmt.Errorf("turn usage query hit its row limit (%d) with an under-cap result — "+
			"treating as unknown rather than risking a false all-clear", rowLimit)
	}

	usage.mu.Lock()
	usage.usedBytes = totalBytes
	usage.overCap = over
	usage.checkedAt = time.Now()
	usage.mu.Unlock()

	usedMB := totalBytes / (1024 * 1024)
	if over {
		return fmt.Sprintf("%d MB used of %d MB cap — TURN credentials suspended for the rest of the month",
			usedMB, config.TurnMonthlyCapMB), nil
	}
	return fmt.Sprintf("%d MB used of %d MB cap", usedMB, config.TurnMonthlyCapMB), nil
}

func initUsageAction() {
	a := &actions.Action{
		ID:          "turn_usage",
		Name:        "TURN usage check",
		Description: "Polls Cloudflare's TURN analytics and suspends new TURN credentials once the monthly cap is hit.",
		Run:         checkUsage,
	}
	actions.Register(a)
	a.SetSchedule(usageCheckInterval)
	go func() { _, _ = a.RunNow() }() // populate the cache once at startup rather than waiting a full interval
}
