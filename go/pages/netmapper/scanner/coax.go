package scanner

import (
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"strconv"
	"sync"
	"time"

	. "tls-rest/go/pages/netmapper/types"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

/*
===============================================================================
MULTI-METRIC COAX ENTRY ENGINE ENGINE CONTROLLER
===============================================================================
*/
func RunCoaxFingerprintMatrix(ips []string, iface string) map[string]*CoaxProfile {
	matrix := make(map[string]*CoaxProfile)
	for _, ip := range ips {
		matrix[ip] = &CoaxProfile{IP: ip, ReceivedTTL: 64}
	}

	var wg sync.WaitGroup

	// Step A: Capture LLDP Signatures on target link (Wired-only frames)
	// We run this asynchronously to listen for topology updates while active queries occur
	lldpMap := make(map[string]bool)
	var lldpMu sync.Mutex
	wg.Add(1)
	go func() {
		defer wg.Done()
		handle, err := pcap.OpenLive(iface, 1600, true, 1*time.Second)
		if err != nil {
			return
		}
		defer handle.Close()
		_ = handle.SetBPFFilter("ether proto 0x88cc or ether proto 0x2000")

		packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
		timeout := time.After(3 * time.Second) // Small bounded lookup window for standard loop evaluation
		for {
			select {
			case packet := <-packetSource.Packets():
				if ethLayer := packet.Layer(layers.LayerTypeEthernet); ethLayer != nil {
					eth, _ := ethLayer.(*layers.Ethernet)
					// Match raw physical link states down the lookup map if matching IP/ARP addresses exist
					lldpMu.Lock()
					lldpMap[eth.SrcMAC.String()] = true
					lldpMu.Unlock()
				}
			case <-timeout:
				return
			}
		}
	}()

	// Step B: Evaluate Latency, TTL, and TCP Window size signatures concurrently across nodes
	for _, ip := range ips {
		wg.Add(1)
		go func(targetIP string) {
			defer wg.Done()

			// 1. Latency metric capture
			minLat := captureMinLatency(targetIP)

			// 2. TTL metric capture
			ttlVal := captureTTL(targetIP)

			// 3. TCP Window metric capture via targeted handshake poke
			tcpWin := captureTCPWindow(targetIP, iface)

			// Consolidate signatures to matrix
			matrix[targetIP].MinLatency = minLat
			matrix[targetIP].ReceivedTTL = ttlVal
			matrix[targetIP].TCPWindowSize = tcpWin
		}(ip)
	}

	wg.Wait()

	// Step C: Compile raw scoring rules to finalize classifications
	for _, profile := range matrix {
		coaxScore := 0

		// Evaluation 1: Hardware physical delay signature (MoCA translation step processing time)
		if profile.MinLatency > 1.2 && profile.MinLatency < 4.1 {
			coaxScore += 35
		}
		// Evaluation 2: Large pipeline sliding structural frame window allocation
		if profile.TCPWindowSize >= 64000 || profile.TCPWindowSize == 65535 {
			coaxScore += 25
		}
		// Evaluation 3: Hidden routed coax interface boundary transition dropping hop bounds by 1
		if profile.ReceivedTTL == 63 || profile.ReceivedTTL == 127 || profile.ReceivedTTL == 254 {
			coaxScore += 30
		}

		// Compile feedback state tags
		if coaxScore >= 50 {
			profile.Classification = "⚠️ Coax / MoCA Segment Node"
		} else if profile.MinLatency >= 4.5 {
			profile.Classification = "Wireless Node"
		} else {
			profile.Classification = "Direct Ethernet Link"
		}
	}

	return matrix
}

/* --- HARDWARE ANALYSIS HELPER FINGERPRINT ENGINE BLOCKS --- */

func captureMinLatency(ip string) float64 {
	re := regexp.MustCompile(`min/avg/max/mdev = ([0-9.]+)/|min/avg/max/stddev = ([0-9.]+)/`)
	out, err := exec.Command("ping", "-c", "10", "-q", "-i", "0.1", ip).Output()
	if err != nil {
		return 0.0
	}
	matches := re.FindStringSubmatch(string(out))
	for i := 1; i < len(matches); i++ {
		if matches[i] != "" {
			val, _ := strconv.ParseFloat(matches[i], 64)
			return val
		}
	}
	return 0.0
}

func captureTTL(ip string) int {
	re := regexp.MustCompile(`ttl=([0-9]+)`)
	out, err := exec.Command("ping", "-c", "1", "-t", "2", ip).Output()
	if err != nil {
		return 64
	}
	matches := re.FindStringSubmatch(string(out))
	if len(matches) > 1 {
		val, _ := strconv.Atoi(matches[1])
		return val
	}
	return 64
}

func captureTCPWindow(ip string, iface string) int {
	handle, err := pcap.OpenLive(iface, 1600, false, 200*time.Millisecond)
	if err != nil {
		return 0
	}
	defer handle.Close()

	filter := fmt.Sprintf("tcp and src host %s and tcp[tcpflags] & (tcp-syn|tcp-ack) == (tcp-syn|tcp-ack)", ip)
	if err := handle.SetBPFFilter(filter); err != nil {
		return 0
	}

	windowChan := make(chan int, 1)
	go func() {
		packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
		select {
		case packet, ok := <-packetSource.Packets():
			if !ok {
				windowChan <- 0
				return
			}
			if tcpLayer := packet.Layer(layers.LayerTypeTCP); tcpLayer != nil {
				tcp, _ := tcpLayer.(*layers.TCP)
				windowChan <- int(tcp.Window)
				return
			}
		case <-time.After(800 * time.Millisecond):
			windowChan <- 0
		}
	}()
	// Poke standard listener profiles to elicit immediate hardware packet structure responses
	go func() {
		d := net.Dialer{Timeout: 300 * time.Millisecond}
		conn, _ := d.Dial("tcp", net.JoinHostPort(ip, "80"))
		if conn != nil {
			conn.Close()
		}
	}()
	return <-windowChan
}
