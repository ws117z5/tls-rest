package accesslog

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"tls-rest/go/engine/controllers/log"
)

// Firewall integration: for access rules flagged `firewall` with action=deny and a
// CIDR, we mirror the block at the host firewall (ufw) so the traffic is dropped
// before it ever reaches the app. This is DECLARATIVE and best-effort — it diffs
// the desired blocks against what it has applied and adds/removes ufw rules to
// match, and if ufw isn't reachable it logs and carries on (see the note about
// containers below).

var (
	fwMu      sync.Mutex
	fwApplied = map[string]bool{} // cidrs we currently have a ufw deny for
)

// ReconcileFirewall makes ufw match the firewall-flagged deny rules. Call it
// after rules are (re)loaded; repeated calls with an unchanged set are no-ops.
func ReconcileFirewall(rules []Rule) {
	fwMu.Lock()
	defer fwMu.Unlock()
	logger := log.For("firewall")

	want := map[string]bool{}
	for _, ru := range rules {
		if ru.Firewall && ru.Action == "deny" && ru.CIDR != "" {
			want[ru.CIDR] = true
		}
	}

	// Add blocks that should exist but don't yet.
	for cidr := range want {
		if fwApplied[cidr] {
			continue
		}
		if err := ufw("deny", "from", cidr); err != nil {
			logger.Warnf("ufw deny from %s failed: %v", cidr, err)
			continue
		}
		logger.Infof("blocked %s at ufw", cidr)
		fwApplied[cidr] = true
	}

	// Remove blocks we added that are no longer wanted.
	for cidr := range fwApplied {
		if want[cidr] {
			continue
		}
		if err := ufw("delete", "deny", "from", cidr); err != nil {
			logger.Warnf("ufw delete deny from %s failed: %v", cidr, err)
			continue
		}
		logger.Infof("unblocked %s at ufw", cidr)
		delete(fwApplied, cidr)
	}
}

func ufw(args ...string) error {
	out, err := exec.Command("ufw", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}