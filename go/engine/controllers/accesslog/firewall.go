package accesslog

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"tls-rest/go/engine/controllers/log"
)

// fwApplied/uaBlocked track ufw deny rules already applied, so reconciling or matching again is a no-op.

var (
	fwMu      sync.Mutex
	fwApplied = map[string]bool{} // cidrs we currently have a ufw deny for

	uaBlockMu sync.Mutex
	uaBlocked = map[string]bool{} // client IPs already blocked for a matched User-Agent rule
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

// blockIPForUARule adds a ufw deny for ip the first time a User-Agent-matched
// (no CIDR) firewall-flagged rule blocks it; later matches for the same ip
// are a no-op so repeated bot traffic doesn't re-run ufw every request.
func blockIPForUARule(ip, userAgent string) {
	uaBlockMu.Lock()
	defer uaBlockMu.Unlock()
	if uaBlocked[ip] {
		return
	}
	comment := fmt.Sprintf("by tls-rest from %s rule, %s", userAgent, time.Now().Format(time.RFC3339))
	if err := ufw("deny", "from", ip, "comment", comment); err != nil {
		// Remembered even on failure: a missing or unprivileged ufw won't fix itself, and retrying would exec and log on every request.
		uaBlocked[ip] = true
		log.For("firewall").Warnf("ufw deny from %s (user-agent rule) failed: %v", ip, err)
		return
	}
	log.For("firewall").Infof("blocked %s at ufw (user-agent rule %q)", ip, userAgent)
	uaBlocked[ip] = true
}

func ufw(args ...string) error {
	out, err := exec.Command("ufw", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}