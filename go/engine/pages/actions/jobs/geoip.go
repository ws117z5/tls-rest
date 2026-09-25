package jobs

import (
	"context"
	"fmt"

	"tls-rest/go/engine/controllers/accesslog"
	actionsctl "tls-rest/go/engine/controllers/actions"
)

func loadGeoIP() (string, error) {
	n4, n6, err := accesslog.LoadGeoIP()
	if err != nil {
		return fmt.Sprintf("%d IPv4 + %d IPv6 ranges loaded", n4, n6), err
	}

	const batch = 5000
	const maxBatches = 50 // caps one run at 250k rows so a stuck loop can't run forever
	var resolved, checked int
	for i := 0; i < maxBatches; i++ {
		r, c, berr := accesslog.BackfillCountriesBatch(context.Background(), batch, "", nil)
		if berr != nil {
			return fmt.Sprintf("%d IPv4 + %d IPv6 ranges loaded; backfill failed after %d rows (%d resolved): %v",
				n4, n6, checked, resolved, berr), berr
		}
		resolved += r
		checked += c
		if c < batch {
			break // fewer rows than asked for: nothing left to backfill
		}
	}
	return fmt.Sprintf("%d IPv4 + %d IPv6 ranges loaded; backfilled %d of %d access_log rows missing a country",
		n4, n6, resolved, checked), nil
}

func init() {
	actionsctl.Register(&actionsctl.Action{
		ID:          "geoip_load",
		Name:        "Load GeoIP tables",
		Description: "Downloads the IPv4/IPv6-to-country range tables (~30MB) from GitHub, then backfills country on every existing access_log row that's missing one.",
		Run:         loadGeoIP,
	})
}
