package httpx

import (
	"net"
	"net/http"
	"strings"
	"sync"
)

var (
	proxyMu       sync.RWMutex
	trustedNets   = privateNets()
	trustAllProxy bool
)

func privateNets() []*net.IPNet {
	var out []*net.IPNet
	for _, c := range []string{"127.0.0.0/8", "::1/128", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "169.254.0.0/16", "fc00::/7"} {
		_, n, _ := net.ParseCIDR(c)
		out = append(out, n)
	}
	return out
}

// SetTrustedProxies installs the proxies whose X-Forwarded-For / X-Real-IP headers are believed, from the TRUSTED_PROXIES env
// (comma-separated IPs/CIDRs; "*" trusts every peer, "none" trusts none; empty keeps the loopback + private-network default).
func SetTrustedProxies(entries []string) {
	var nets []*net.IPNet
	all := false
	for _, e := range entries {
		e = strings.TrimSpace(e)
		switch {
		case e == "":
		case e == "*":
			all = true
		case strings.EqualFold(e, "none"):
			nets = []*net.IPNet{}
		default:
			if _, n, err := net.ParseCIDR(e); err == nil {
				nets = append(nets, n)
			} else if ip := net.ParseIP(e); ip != nil {
				bits := 128
				if ip.To4() != nil {
					bits = 32
				}
				nets = append(nets, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
			}
		}
	}
	proxyMu.Lock()
	defer proxyMu.Unlock()
	trustAllProxy = all
	if nets != nil {
		trustedNets = nets
	}
}

func isTrustedProxy(ip net.IP) bool {
	proxyMu.RLock()
	defer proxyMu.RUnlock()
	if trustAllProxy {
		return true
	}
	for _, n := range trustedNets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// ClientIP is the caller's address: the connection peer, unless that peer is a trusted proxy, in which case
// X-Forwarded-For is walked right to left and the first hop that is not itself a trusted proxy wins (a client cannot spoof what it appends to the left).
func ClientIP(r *http.Request) string {
	peer := r.RemoteAddr
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		peer = host
	}
	pip := net.ParseIP(peer)
	if pip == nil || !isTrustedProxy(pip) {
		return peer
	}

	hops := strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
	for i := len(hops) - 1; i >= 0; i-- {
		hop := strings.TrimSpace(hops[i])
		ip := net.ParseIP(hop)
		if ip == nil {
			continue
		}
		if !isTrustedProxy(ip) {
			return hop
		}
	}
	if xr := net.ParseIP(strings.TrimSpace(r.Header.Get("X-Real-IP"))); xr != nil {
		return xr.String()
	}
	if len(hops) > 0 {
		if first := net.ParseIP(strings.TrimSpace(hops[0])); first != nil {
			return first.String()
		}
	}
	return peer
}
