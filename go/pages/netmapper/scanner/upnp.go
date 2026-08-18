package scanner

import (
	"log"
	"net"
	"strings"
	"time"
)

func DiscoverUPnPMediaDevices() []string {
	var discoveredIPs []string
	localAddr, err := net.ResolveUDPAddr("udp4", "0.0.0.0:0")
	if err != nil {
		return discoveredIPs
	}

	conn, err := net.ListenUDP("udp4", localAddr)
	if err != nil {
		return discoveredIPs
	}
	defer conn.Close()

	destAddr, err := net.ResolveUDPAddr("udp4", "239.255.255.250:1900")
	if err != nil {
		return discoveredIPs
	}

	ssdpQuery := "M-SEARCH * HTTP/1.1\r\n" +
		"HOST: 239.255.255.250:1900\r\n" +
		"MAN: \"ssdp:discover\"\r\n" +
		"MX: 2\r\n" +
		"ST: ssdp:all\r\n\r\n"

	_ = conn.SetWriteDeadline(time.Now().Add(1 * time.Second))
	_, _ = conn.WriteToUDP([]byte(ssdpQuery), destAddr)

	_ = conn.SetReadDeadline(time.Now().Add(1 * time.Second).Add(50 * time.Millisecond))
	buffer := [2048]byte{}
	ipMap := make(map[string]bool)

	for {
		n, remoteAddr, err := conn.ReadFromUDP(buffer[0:])
		if err != nil {
			break
		}
		resp := string(buffer[:n])
		if strings.Contains(resp, "HTTP/1.1 200 OK") || strings.Contains(resp, "NOTIFY") {
			ip := remoteAddr.IP.String()
			if !ipMap[ip] {
				ipMap[ip] = true
				discoveredIPs = append(discoveredIPs, ip)
				log.Printf("[+] UPnP/SSDP Discovery found boundary device: %s\n", ip)
			}
		}
	}
	return discoveredIPs
}
