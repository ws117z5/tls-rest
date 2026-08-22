// Package mesh integrates the measured-overlay optimizer (the balancer +
// resolver) into the papers WebRTC feature. Peers report the links they
// measured to other peers; the coordinator turns those reports into the N×N
// matrices the optimizer needs, runs BuildPlan (Frank-Wolfe balancing +
// per-source distribution trees), and hands back a plan telling every peer what
// to publish, relay and pull.
package mesh

import (
	"sync"

	"tls-rest/go/engine/controllers/optimizer"
)

// LinkStat is one measured link from the reporting peer to Peer.
type LinkStat struct {
	Peer      string  `json:"peer"`
	LatencyMs float64 `json:"latencyMs"`
	UpMbps    float64 `json:"upMbps"`
}

// Report is a single peer's self-measurement plus the links it observed.
type Report struct {
	Up    float64    `json:"up"`   // this peer's uplink capacity (Mbit/s)
	Down  float64    `json:"down"` // this peer's downlink capacity (Mbit/s)
	Stats []LinkStat `json:"stats"`
}

// PlanEnvelope is what clients pull from GET /papers/{room}/plan: the peer order
// (so tree indices map back to peer ids) and the optimizer plan.
type PlanEnvelope struct {
	Order             []string        `json:"order"`
	Plan              *optimizer.Plan `json:"plan"`
	StreamBitrateKbps int             `json:"streamBitrateKbps"`
}

// Coordinator holds the live reports for every room and rebuilds a plan on
// demand. It is safe for concurrent use.
type Coordinator struct {
	mu    sync.Mutex
	rooms map[string]*roomState
}

type roomState struct {
	order   []string           // stable peer ordering -> optimizer indices
	reports map[string]*Report // peerID -> latest report
}

func NewCoordinator() *Coordinator {
	return &Coordinator{rooms: make(map[string]*roomState)}
}

func (c *Coordinator) room(roomID string) *roomState {
	rs, ok := c.rooms[roomID]
	if !ok {
		rs = &roomState{reports: make(map[string]*Report)}
		c.rooms[roomID] = rs
	}
	return rs
}

// Report records a peer's measurements, adding it to the room ordering the first
// time it is seen.
func (c *Coordinator) Report(roomID, peerID string, rep *Report) {
	c.mu.Lock()
	defer c.mu.Unlock()
	rs := c.room(roomID)
	if _, seen := rs.reports[peerID]; !seen {
		rs.order = append(rs.order, peerID)
	}
	rs.reports[peerID] = rep
}

// Remove drops a peer that has left the room.
func (c *Coordinator) Remove(roomID, peerID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	rs, ok := c.rooms[roomID]
	if !ok {
		return
	}
	delete(rs.reports, peerID)
	for i, id := range rs.order {
		if id == peerID {
			rs.order = append(rs.order[:i], rs.order[i+1:]...)
			break
		}
	}
	if len(rs.order) == 0 {
		delete(c.rooms, roomID)
	}
}

// Plan builds a relay plan for the room from the reports gathered so far.
// bitrateKbps is the per-stream media bitrate. It returns (nil, waiting) when
// fewer than two peers are present or a present peer has not reported yet;
// waiting lists the peers still missing.
func (c *Coordinator) Plan(roomID string, bitrateKbps int) (*PlanEnvelope, []string) {
	c.mu.Lock()
	rs, ok := c.rooms[roomID]
	if !ok {
		c.mu.Unlock()
		return nil, nil
	}
	order := append([]string(nil), rs.order...)
	reports := make(map[string]*Report, len(rs.reports))
	for id, r := range rs.reports {
		reports[id] = r
	}
	c.mu.Unlock()

	return BuildPlanFor(order, reports, bitrateKbps)
}

// BuildPlanFor is the pure transformation reports -> optimizer input -> plan. It
// is exported so it can be exercised directly in tests without a Coordinator.
func BuildPlanFor(order []string, reports map[string]*Report, bitrateKbps int) (*PlanEnvelope, []string) {
	n := len(order)
	if n < 2 {
		return nil, nil
	}

	idx := make(map[string]int, n)
	for i, id := range order {
		idx[id] = i
	}

	// Every present peer must have reported at least once.
	var waiting []string
	for _, id := range order {
		if reports[id] == nil {
			waiting = append(waiting, id)
		}
	}
	if len(waiting) > 0 {
		return nil, waiting
	}

	in := blankInput(n)
	for i, id := range order {
		rep := reports[id]
		in.Up[i] = orDefault(rep.Up, 10)
		in.Down[i] = orDefault(rep.Down, 50)
		for _, s := range rep.Stats {
			j, ok := idx[s.Peer]
			if !ok || j == i {
				continue
			}
			in.Lat[i][j] = s.LatencyMs
			in.BW[i][j] = maxf(s.UpMbps, 0.1)
		}
	}

	streamMbps := float64(bitrateKbps) / 1000.0
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if i != j {
				in.Demand[i][j] = streamMbps
			}
		}
	}

	plan := optimizer.BuildPlan(in)
	return &PlanEnvelope{Order: order, Plan: plan, StreamBitrateKbps: bitrateKbps}, nil
}

func blankInput(n int) *optimizer.Input {
	mk := func() [][]float64 {
		m := make([][]float64, n)
		for i := range m {
			m[i] = make([]float64, n)
		}
		return m
	}
	in := &optimizer.Input{
		N: n, Lat: mk(), BW: mk(), Demand: mk(),
		Up: make([]float64, n), Down: make([]float64, n),
		MaxRelayHops: 1,
	}
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if i != j {
				in.Lat[i][j] = 500 // pessimistic until measured
			}
		}
	}
	return in
}

func orDefault(v, d float64) float64 {
	if v <= 0 {
		return d
	}
	return v
}

func maxf(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
