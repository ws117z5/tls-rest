package resolver

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	. "tls-rest/go/pages/netmapper/types"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

func CheckModemAndResolveType(ip string, timeout time.Duration) ModemInfo {
	target := net.JoinHostPort(ip, "80")
	start := time.Now()
	conn, err := net.DialTimeout("tcp", target, timeout)
	if err != nil {
		target = net.JoinHostPort(ip, "443")
		conn, err = net.DialTimeout("tcp", target, timeout)
	}

	if err != nil {
		return ModemInfo{
			IP:      "Direct Fiber / Unmanaged Edge",
			Status:  "Standard WAN / Fiber Gateway",
			Latency: "N/A",
		}
	}
	conn.Close()
	return ModemInfo{
		IP:      ip,
		Status:  "Coaxial HFC Link (DOCSIS Cable Modem)",
		Latency: time.Since(start).String(),
	}
}

func RunTracerouteWithEnrichment(target string, maxHops int, sendEvent func(string, interface{})) []HopInfo {
	var hops []HopInfo
	destAddr, err := net.ResolveIPAddr("ip4", target)
	if err != nil {
		return hops
	}

	c, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		log.Printf("[!] Failed to listen on raw ICMP socket: %v\n", err)
		return hops
	}
	defer c.Close()

	for ttl := 1; ttl <= maxHops; ttl++ {
		_ = c.IPv4PacketConn().SetTTL(ttl)

		body := &icmp.Echo{
			ID:   1337,
			Seq:  ttl,
			Data: []byte("NET-MAPPING"),
		}
		msg := icmp.Message{
			Type: ipv4.ICMPTypeEcho,
			Code: 0,
			Body: body,
		}

		bytes, err := msg.Marshal(nil)
		if err != nil {
			continue
		}

		start := time.Now()
		_, err = c.WriteTo(bytes, destAddr)
		if err != nil {
			continue
		}

		_ = c.SetReadDeadline(time.Now().Add(800 * time.Millisecond))
		reply := make([]byte, 1500)
		n, peer, err := c.ReadFrom(reply)
		if err != nil {
			continue
		}

		latency := time.Since(start)
		peerIP := peer.String()

		rm, err := icmp.ParseMessage(1, reply[:n])
		if err != nil {
			continue
		}

		hostname := "Unknown"
		names, err := net.LookupAddr(peerIP)
		if err == nil && len(names) > 0 {
			hostname = strings.TrimSuffix(names[0], ".")
		}

		org := "Carrier Backbone"
		func() {
			ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
			defer cancel()

			req, _ := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("https://ipapi.co/%s/org/", peerIP), nil)
			req.Header.Set("User-Agent", "curl/7.68.0")
			resp, err := http.DefaultClient.Do(req)
			if err == nil {
				defer resp.Body.Close()
				bodyBytes, err := io.ReadAll(resp.Body)
				if err == nil && len(bodyBytes) > 0 {
					orgText := strings.TrimSpace(string(bodyBytes))
					if !strings.Contains(orgText, "<html>") && len(orgText) < 60 {
						org = orgText
					}
				}
			}
		}()

		log.Printf("[Hop %d] IP: %s | Hostname: %s | Org: %s | Latency: %s\n", ttl, peerIP, hostname, org, latency)

		hop := HopInfo{
			TTL:      ttl,
			IP:       peerIP,
			Latency:  latency.String(),
			Hostname: hostname,
			Org:      org,
		}
		hops = append(hops, hop)
		sendEvent("hop_discovered", hop)

		if rm.Type == ipv4.ICMPTypeEchoReply {
			break
		}
	}
	return hops
}
