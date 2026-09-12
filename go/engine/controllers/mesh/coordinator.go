// Package mesh integrates the measured-overlay optimizer (the balancer +
// resolver) into the papers WebRTC feature. Peers report the links they
// measured to other peers; the coordinator turns those reports into the N×N
// matrices the optimizer needs, runs BuildPlan (Frank-Wolfe balancing +
// per-source distribution trees), and hands back a plan telling every peer what
// to publish, relay and pull.
package mesh

import (
	"sort"
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
	Order []string        `json:"order"`
	Plan  *optimizer.Plan `json:"plan"`
	// VideoTiers is each peer's own resolution/bitrate to publish at, keyed by peer id (see BuildPlanFor).
	VideoTiers map[string]VideoTier `json:"videoTiers"`
}

// VideoTier is one selectable publish quality. Width/Height are advisory
// capture dimensions (applied via the browser's own aspect-preserving
// downscale); BitrateKbps is what the balancer sizes per-stream demand from.
type VideoTier struct {
	Width       int `json:"width"`
	Height      int `json:"height"`
	BitrateKbps int `json:"bitrateKbps"`
}

// videoTiers are tried highest quality first. maxSafeUtil is the utilization
// ceiling (of uplink/downlink/relay capacity) a tier's plan must fit under,
// with headroom below 1.0 so a room doesn't sit right at the edge of dropping
// frames — trading resolution for a stable frame rate is the point.
var videoTiers = []VideoTier{
	{Width: 1280, Height: 720, BitrateKbps: 1200},
	{Width: 854, Height: 480, BitrateKbps: 600},
	{Width: 640, Height: 360, BitrateKbps: 300},
	{Width: 320, Height: 240, BitrateKbps: 150},
}

const maxSafeUtil = 0.85

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

// Plan builds a relay plan for the room from the reports gathered so far. It
// returns (nil, waiting) when fewer than two peers are present or a present
// peer has not reported yet; waiting lists the peers still missing.
func (c *Coordinator) Plan(roomID string) (*PlanEnvelope, []string) {
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

	return BuildPlanFor(order, reports)
}

// BuildPlanFor is the pure transformation reports -> optimizer input -> plan.
// It is exported so it can be exercised directly in tests without a
// Coordinator.
//
// Each source's row of the demand matrix is sized from ITS OWN video tier, so
// publishers are capped independently rather than the whole room sharing one
// bitrate. Tiers are assigned greedily: every publisher starts at the lowest
// (always safe) tier, then — highest measured uplink first, since they have
// the most headroom to spend — each is bumped up one tier at a time as far as
// the shared mesh can sustain without breaching maxSafeUtil. A publisher's own
// uplink funds this, but relay/downlink capacity on links their stream shares
// with others is pooled across every source in the room, not sizeable in
// isolation — hence the shared rebuild-and-check per bump rather than each
// publisher just picking a tier off its own numbers.
func BuildPlanFor(order []string, reports map[string]*Report) (*PlanEnvelope, []string) {
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

	fits := func(res *optimizer.Result) bool {
		return res.MaxUpUtil <= maxSafeUtil && res.MaxDownUtil <= maxSafeUtil && res.MaxRelayUtil <= maxSafeUtil
	}
	lowest := len(videoTiers) - 1
	tierIdx := make([]int, n) // per source: index into videoTiers, 0 = highest
	for i := range tierIdx {
		tierIdx[i] = lowest
	}
	setDemand := func() {
		for i := 0; i < n; i++ {
			mbps := float64(videoTiers[tierIdx[i]].BitrateKbps) / 1000.0
			for j := 0; j < n; j++ {
				if i != j {
					in.Demand[i][j] = mbps
				}
			}
		}
	}
	envelopeFor := func(plan *optimizer.Plan) *PlanEnvelope {
		tiers := make(map[string]VideoTier, n)
		for i, id := range order {
			tiers[id] = videoTiers[tierIdx[i]]
		}
		return &PlanEnvelope{Order: order, Plan: plan, VideoTiers: tiers}
	}

	setDemand()
	plan := optimizer.BuildPlan(in)
	if !fits(plan.Result) {
		// Even everyone at the lowest tier doesn't fit: ship it anyway, it's
		// the best available, and reporting keeps refining as conditions change.
		return envelopeFor(plan), nil
	}

	priority := make([]int, n)
	for i := range priority {
		priority[i] = i
	}
	sort.Slice(priority, func(a, b int) bool { return in.Up[priority[a]] > in.Up[priority[b]] })

	for _, i := range priority {
		for tierIdx[i] > 0 {
			trial := tierIdx[i] - 1
			tierIdx[i] = trial
			setDemand()
			candidate := optimizer.BuildPlan(in)
			if !fits(candidate.Result) {
				tierIdx[i] = trial + 1 // revert the bump that broke it
				setDemand()
				break
			}
			plan = candidate
		}
	}

	return envelopeFor(plan), nil
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
