package accesslog

import (
	"net"
	"strings"
	"sync"
	"time"

	"tls-rest/go/engine/controllers/db/pgdb"
	"tls-rest/go/engine/controllers/functions"
)

// Rule is one IP/CIDR access-control rule loaded from the access_rule table.
type Rule struct {
	ID        int64
	Net       *net.IPNet // parsed from cidr; nil = matches any IP (UA-only rule)
	CIDR      string     // original cidr text (for firewall rules)
	UserAgent string     // substring matched against the request User-Agent; "" = any
	Action    string     // "allow" | "deny"
	Priority  int
	Firewall  bool // also enforce at the OS firewall (ufw) — CIDR deny rules only
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
func Decision(ipStr, userAgent string) (bool, int64) {
	ip := parseIP(ipStr)

	for _, ru := range currentRules() {
		// A rule must constrain by IP and/or User-Agent; skip empty rules.
		if ru.Net == nil && ru.UserAgent == "" {
			continue
		}
		// IP constraint (if any): must contain the client IP.
		if ru.Net != nil {
			if ip == nil || !ru.Net.Contains(ip) {
				continue
			}
		}
		// User-Agent constraint (if any): substring match, case-sensitive.
		if ru.UserAgent != "" && !strings.Contains(userAgent, ru.UserAgent) {
			continue
		}
		return ru.Action == "allow", ru.ID
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
	rows, err := db.GetAll(`SELECT id, cidr, action, priority, user_agent, firewall FROM access_rule WHERE enabled = true ORDER BY priority ASC, id ASC`)
	if err != nil {
		markLoaded()
		return currentSnapshot()
	}

	parsed := make([]Rule, 0, len(rows))
	for _, row := range rows {
		cidr := functions.Coerce[string](row["cidr"])
		ua := functions.Coerce[string](row["user_agent"])
		n := parseCIDR(cidr)
		// A rule needs a CIDR and/or a User-Agent; skip if it has neither.
		if n == nil && ua == "" {
			continue
		}
		action := functions.Coerce[string](row["action"])
		if action != "allow" {
			action = "deny"
		}
		parsed = append(parsed, Rule{
			ID:        functions.Coerce[int64](row["id"]),
			Net:       n,
			CIDR:      cidr,
			UserAgent: ua,
			Action:    action,
			Priority:  int(functions.Coerce[int64](row["priority"])),
			Firewall:  functions.Coerce[bool](row["firewall"]),
		})
	}

	rulesMu.Lock()
	rules = parsed
	rulesLoaded = time.Now()
	rulesMu.Unlock()

	// Keep the OS firewall (ufw) in sync with firewall-flagged deny rules. Async
	// and best-effort so it never blocks a request; no-op when nothing changed.
	go ReconcileFirewall(parsed)

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
