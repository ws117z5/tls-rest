package types

type StreamEvent struct {
	Step    string      `json:"step"`
	Payload interface{} `json:"payload"`
}

type ModemInfo struct {
	IP      string `json:"ip"`
	Status  string `json:"status"`
	Latency string `json:"latency"`
}

type DeviceNode struct {
	IP           string `json:"ip"`
	Type         string `json:"type"`
	OpenPorts    []int  `json:"open_ports"`
	CoaxFeedback string `json:"coax_feedback,omitempty"` // Add this to tie metrics back to nodes
}

type HopInfo struct {
	TTL      int    `json:"ttl"`
	IP       string `json:"ip"`
	Latency  string `json:"latency"`
	Hostname string `json:"hostname"`
	Org      string `json:"org"`
}

// CoaxProfile contains the combined physical layer signatures for a single LAN device
type CoaxProfile struct {
	IP             string  `json:"ip"`
	MinLatency     float64 `json:"min_latency_ms"`
	LLDPDetected   bool    `json:"lldp_detected"`
	ReceivedTTL    int     `json:"received_ttl"`
	TCPWindowSize  int     `json:"tcp_window_size"`
	Classification string  `json:"classification"` // "Coax / MoCA Node", "Direct Ethernet", or "Wireless"
}
