package accesslog

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"tls-rest/go/engine/controllers/db/pgdb"
	"tls-rest/go/engine/controllers/functions"
	"tls-rest/go/engine/controllers/log"
)

// ipRange is one [start,end] IPv4/IPv6 block mapped to a country. Addresses
// are compared as raw big-endian bytes (4 bytes for v4, 16 for v6), so both
// families share the same range/search logic.
type ipRange struct {
	start, end []byte
	country    string
}

var (
	geoMu   sync.RWMutex
	ranges4 []ipRange
	ranges6 []ipRange
)

// countryDataURLs are the sapics/ip-location-db (DB-IP-sourced) country CSVs,
// fetched directly from GitHub on demand — never vendored into this repo.
var countryDataURLs = []string{
	"https://raw.githubusercontent.com/sapics/ip-location-db/main/dbip-country/dbip-country-ipv4.csv",
	"https://raw.githubusercontent.com/sapics/ip-location-db/main/dbip-country/dbip-country-ipv6.csv",
}

// LoadGeoIP downloads and parses both country range tables (~30MB combined),
// blocking until done — an explicit, occasional admin action (see the
// Statistics page), not an automatic per-process warm-up. Safe to call again
// later to refresh; returns the range count loaded for each family.
func LoadGeoIP() (n4, n6 int, err error) {
	logger := log.For("geoip")
	for _, url := range countryDataURLs {
		ranges, ferr := fetchCountryRanges(url)
		if ferr != nil {
			logger.Warnf("fetch %s: %v", url, ferr)
			err = ferr
			continue
		}
		sort.Slice(ranges, func(i, j int) bool { return bytes.Compare(ranges[i].start, ranges[j].start) < 0 })
		geoMu.Lock()
		if strings.Contains(url, "ipv6") {
			ranges6 = ranges
		} else {
			ranges4 = ranges
		}
		geoMu.Unlock()
		logger.Infof("loaded %d ranges from %s", len(ranges), url)
	}
	geoMu.RLock()
	n4, n6 = len(ranges4), len(ranges6)
	geoMu.RUnlock()
	return n4, n6, err
}

// GeoIPLoaded reports whether either range table currently holds data.
func GeoIPLoaded() bool {
	geoMu.RLock()
	defer geoMu.RUnlock()
	return len(ranges4) > 0 || len(ranges6) > 0
}

// fetchCountryRanges downloads and parses one "start,end,country" CSV.
func fetchCountryRanges(url string) ([]ipRange, error) {
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var out []ipRange
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		parts := strings.SplitN(scanner.Text(), ",", 3)
		if len(parts) != 3 {
			continue
		}
		start := net.ParseIP(parts[0])
		end := net.ParseIP(parts[1])
		if start == nil || end == nil {
			continue
		}
		out = append(out, ipRange{start: normalizeIP(start), end: normalizeIP(end), country: parts[2]})
	}
	return out, scanner.Err()
}

// normalizeIP returns the 4-byte form for an IPv4 address and the 16-byte
// form for IPv6, so byte-slice comparisons are family-consistent.
func normalizeIP(ip net.IP) []byte {
	if v4 := ip.To4(); v4 != nil {
		return v4
	}
	return ip.To16()
}

// CountryForIP returns the 2-letter country code for ipStr, or "" when
// nothing is loaded yet (see LoadGeoIP), the IP is unparsable, or no range matches.
func CountryForIP(ipStr string) string {
	ip := net.ParseIP(stripPort(ipStr))
	if ip == nil {
		return ""
	}
	needle := normalizeIP(ip)

	geoMu.RLock()
	table := ranges4
	if len(needle) == 16 {
		table = ranges6
	}
	geoMu.RUnlock()
	if len(table) == 0 {
		return ""
	}

	i := sort.Search(len(table), func(i int) bool { return bytes.Compare(table[i].start, needle) > 0 })
	if i == 0 {
		return ""
	}
	r := table[i-1]
	if bytes.Compare(needle, r.end) <= 0 {
		return r.country
	}
	return ""
}

// BackfillCountriesBatch resolves country for up to limit access_log rows
// missing one (optionally narrowed by an extra WHERE fragment, "" for none),
// using whichever GeoIP tables are loaded. checked > resolved means some IPs
// matched no range.
func BackfillCountriesBatch(ctx context.Context, limit int, extraWhere string, extraArgs []interface{}) (resolved, checked int, err error) {
	db, err := pgdb.GetInstanceCtx(ctx)
	if err != nil {
		return 0, 0, err
	}
	where := "WHERE country IS NULL AND ip IS NOT NULL"
	if extraWhere != "" {
		where += " AND " + extraWhere
	}
	rows, err := db.GetAll(fmt.Sprintf("SELECT id, ip FROM access_log %s LIMIT %d", where, limit), extraArgs...)
	if err != nil {
		return 0, 0, err
	}
	for _, row := range rows {
		id := functions.Coerce[int64](row["id"])
		ip := functions.Coerce[string](row["ip"])
		country := CountryForIP(ip)
		if country == "" {
			continue
		}
		if _, err := db.Exec("UPDATE access_log SET country = $1 WHERE id = $2", country, id); err == nil {
			resolved++
		}
	}
	return resolved, len(rows), nil
}

// stripPort drops a ":port" suffix from a bare IPv4 host, if present (IPv6
// addresses without brackets have no unambiguous port form, so left as-is).
func stripPort(s string) string {
	if i := strings.LastIndexByte(s, ':'); i >= 0 && strings.Count(s, ":") == 1 {
		return s[:i]
	}
	return s
}
