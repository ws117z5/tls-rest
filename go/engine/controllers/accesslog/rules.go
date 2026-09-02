package accesslog

import (
	"net"
	"sync"
	"time"

	"tls-rest/go/engine/controllers/db/pgdb"
	"tls-rest/go/engine/controllers/functions"
)

// Rule is one IP/CIDR access-control rule loaded from the access_rule table.
type Rule struct {
	ID       int64
	Net      *net.IPNet // parsed from cidr (a bare IP becomes a /32 or /128)
	Action   string     // "allow" | "deny"
	Priority int
}

var (
	rulesMu      sync.RWMutex
	rules        []Rule
	rulesLoaded  time.Time
	rulesTTL     = 30 * time.Second // rules are re-read at most this often
	rulesRefresh sync.Once
)

// Decision resolves an IP against the loaded rules. Rules are evaluated in
// priority order (lowest first) and the first CIDR that contains the IP decides:
// its action allows or blocks the request. With no matching rule the request is
// allowed, so the site works with an empty table and supports both denylists
// (add 'deny' rules) and allowlists (a low-priority 'deny 0.0.0.0/0' plus
// higher-priority 'allow' rules).
//
// Returns (allowed, matchedRuleID). matchedRuleID is 0 when nothing matched.
func Decision(ipStr string) (bool, int64) {
	ip := parseIP(ipStr)
	if ip == nil {
		return true, 0 // unparseable → don't block
	}

	for _, ru := range currentRules() {
		if ru.Net != nil && ru.Net.Contains(ip) {
			return ru.Action == "allow", ru.ID
		}
	}
	return true, 0
}

// currentRules returns the cached rule set, refreshing it from the DB when the
// TTL has expired. Never blocks a request on a DB error — it keeps the last set.
func currentRules() []Rule {
	rulesMu.RLock()
	fresh := time.Since(rulesLoaded) < rulesTTL && rulesLoaded.IsZero() == false
	snapshot := rules
	rulesMu.RUnlock()
	if fresh {
		return snapshot
	}
	return ReloadRules()
}

// ReloadRules reads enabled rules from the DB and replaces the cache. Exposed so
// the access-control page can force a refresh after edits.
func ReloadRules() []Rule {
	db, err := pgdb.GetInstance()
	if err != nil {
		markLoaded()
		return currentSnapshot()
	}
	rows, err := db.GetAll(`SELECT id, cidr, action, priority FROM access_rule WHERE enabled = true ORDER BY priority ASC, id ASC`)
	if err != nil {
		markLoaded()
		return currentSnapshot()
	}

	parsed := make([]Rule, 0, len(rows))
	for _, row := range rows {
		cidr := functions.Coerce[string](row["cidr"])
		n := parseCIDR(cidr)
		if n == nil {
			continue
		}
		action := functions.Coerce[string](row["action"])
		if action != "allow" {
			action = "deny"
		}
		parsed = append(parsed, Rule{
			ID:       functions.Coerce[int64](row["id"]),
			Net:      n,
			Action:   action,
			Priority: int(functions.Coerce[int64](row["priority"])),
		})
	}

	rulesMu.Lock()
	rules = parsed
	rulesLoaded = time.Now()
	rulesMu.Unlock()
	return parsed
}

func currentSnapshot() []Rule {
	rulesMu.RLock()
	defer rulesMu.RUnlock()
	return rules
}

func markLoaded() {
	rulesMu.Lock()
	rulesLoaded = time.Now()
	rulesMu.Unlock()
}

// parseCIDR accepts either a CIDR ("10.0.0.0/8") or a bare IP ("10.1.2.3"),
// returning the network. A bare IP becomes a host route (/32 or /128).
func parseCIDR(s string) *net.IPNet {
	if _, n, err := net.ParseCIDR(s); err == nil {
		return n
	}
	if ip := net.ParseIP(s); ip != nil {
		bits := 32
		if ip.To4() == nil {
			bits = 128
		}
		return &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)}
	}
	return nil
}

// parseIP tolerates a bare IP or "ip:port".
func parseIP(s string) net.IP {
	if ip := net.ParseIP(s); ip != nil {
		return ip
	}
	if host, _, err := net.SplitHostPort(s); err == nil {
		return net.ParseIP(host)
	}
	return nil
}
