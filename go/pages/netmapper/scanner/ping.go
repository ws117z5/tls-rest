package scanner

import (
	"net"
	"sync"
	"time"
)

func PerformPingSweep(subnetCIDR string) {
	ip, ipNet, err := net.ParseCIDR(subnetCIDR)
	if err != nil {
		return
	}

	var wg sync.WaitGroup
	for currentIP := ip.Mask(ipNet.Mask); ipNet.Contains(currentIP); incIP(currentIP) {
		targetIP := currentIP.String()
		wg.Add(1)

		go func(host string) {
			defer wg.Done()
			conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, "80"), 80*time.Millisecond)
			if err == nil {
				conn.Close()
				return
			}
			conn2, err := net.DialTimeout("tcp", net.JoinHostPort(host, "443"), 80*time.Millisecond)
			if err == nil {
				conn2.Close()
			}
		}(targetIP)
	}
	wg.Wait()
}

func incIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}
