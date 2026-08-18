package scanner

import (
	"net"
	"os/exec"
	"strings"

	. "tls-rest/go/pages/netmapper/types"
)

func GetArpTableDevices() []DeviceNode {
	var devices []DeviceNode
	cmd := exec.Command("arp", "-a")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return devices
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			rawIP := fields[1]
			ipStr := strings.Trim(rawIP, "()")
			parsedIP := net.ParseIP(ipStr)
			if parsedIP != nil && parsedIP.To4() != nil {
				if !strings.HasSuffix(ipStr, ".255") && ipStr != "127.0.0.1" {
					devType := "LAN Node"
					if strings.HasSuffix(ipStr, ".1") {
						devType = "Local Gateway / Coax Bridge"
					}
					devices = append(devices, DeviceNode{
						IP:        ipStr,
						Type:      devType,
						OpenPorts: []int{},
					})
				}
			}
		}
	}
	return devices
}
