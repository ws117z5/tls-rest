// Package mesh is the reusable WebRTC mesh controller (relay planning via
// Frank-Wolfe balancing, plus signaling relay), backing papers' video mesh.
package mesh

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

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
	// Platform tags the client type ("web", "android", "ios"); empty/"web" keeps
	// the historical unlimited relay cap, anything else gets mobileRelayCapMbps
	// (see BuildPlanFor) — relaying is real decode+re-encode CPU/battery cost on
	// a phone, not packet forwarding, which an unlimited cap doesn't account for.
	Platform string `json:"platform"`
}

// mobileRelayCapMbps is a conservative placeholder for a phone's sustained relay
// (decode+re-encode) throughput budget — not benchmarked, meant to be tuned from
// real telemetry once this is live on mobile.
const mobileRelayCapMbps = 4.0

// platformRelayCapMbps overrides mobileRelayCapMbps for a specific non-"web"
// platform tag — "server" is the Go backend's own synthetic relay peer (see
// Coordinator.Plan/withServerReport); any other non-empty/non-"web" tag
// keeps the historical mobileRelayCapMbps fallback.
var platformRelayCapMbps = map[string]float64{
	"server": serverRelayCapMbps,
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

// staleAfter is how long a peer can go without reporting before Plan evicts
// it. Both clients (web's probe.ts, Android/the Rust rig) re-report every
// ~6s, so 3 missed cycles is a real disconnect, not jitter — a crashed or
// killed client can never call Remove itself, so without this a dead peer's
// last-known numbers get planned around forever (only an empty room is ever
// garbage collected otherwise).
const staleAfter = 20 * time.Second

// ServerPeerID is the Go backend's own reserved peer id in every room's
// mesh — "0", which real user ids (positive Postgres auto-increment ints)
// can never collide with, and which sorts lexicographically below every
// such id, so under the mesh.ts/CallMesh.kt selfId > peerId polite/impolite
// convention the server is always impolite (its offers always win a
// collision, the human side always rolls back) — an arbitrary but fine,
// deterministic tie-break.
const ServerPeerID = "0"

// serverUpMbps/serverDownMbps/serverRelayCapMbps size the synthetic report
// BuildPlanFor sees for the server's own relay peer (see withServerReport) —
// generous placeholders, not benchmarked against the real deployment's
// actual link, meant to be tuned from real telemetry once ReportServerLink
// has real numbers to feed back.
const (
	serverUpMbps       = 1000.0
	serverDownMbps     = 1000.0
	serverRelayCapMbps = 200.0
)

// engageServerUtilThreshold/engageServerPeerCount decide, per room per Plan
// call, whether the server should join a room as an extra relay node: it
// joins if human-only utilization would reach the threshold, or the room
// already has this many peers. disengageServerUtilThreshold is the lower
// bar a room must drop back under (hysteresis) before the server leaves
// again, so a room sitting right at the boundary doesn't flap it on/off
// every Plan cycle.
const (
	engageServerUtilThreshold    = 0.7
	disengageServerUtilThreshold = 0.5
	engageServerPeerCount        = 5
)

// Coordinator holds the live reports for every room and rebuilds a plan on
// demand. It is safe for concurrent use.
type Coordinator struct {
	mu                 sync.Mutex
	rooms              map[string]*roomState
	serverRelayEnabled bool
}

type roomState struct {
	order         []string             // stable peer ordering -> optimizer indices
	reports       map[string]*Report   // peerID -> latest report
	lastSeen      map[string]time.Time // peerID -> time of its latest report
	serverEngaged bool                 // whether ServerPeerID was injected into the last Plan() for this room
	serverLinks   map[string]LinkStat  // peerID -> last real measured server<->peer link (see ReportServerLink)

	generation     int            // bumped by every Report() call — marks "something actually changed"
	lastHistoryGen int            // generation last recorded into history, so repeated/polling Plan() calls don't duplicate it
	history        []historyPoint // bounded recent plans, oldest first — see appendHistory/Timeline
}

// historyPoint is one recorded Plan() outcome for a room, kept so Timeline
// can reconstruct that moment's adjacency matrix later (see buildRoomMatrix)
// — Reports is a snapshot of what each peer had reported as of Env, since
// per-pair latency/bandwidth measurements live there, not in Env itself.
type historyPoint struct {
	Time    time.Time
	Env     *PlanEnvelope
	Reports map[string]*Report
}

// maxHistoryPoints bounds each room's kept history — old points fall off the
// front once exceeded, so a long-running room can't grow this without limit.
const maxHistoryPoints = 200

func NewCoordinator() *Coordinator {
	return &Coordinator{rooms: make(map[string]*roomState), serverRelayEnabled: true}
}

// SetServerRelayEnabled is the operational kill switch for the server
// joining any room as an extra relay node (see Plan) — defaults on;
// flipping it off stops ServerPeerID from ever being injected, regardless
// of room conditions, with no deploy needed.
func (c *Coordinator) SetServerRelayEnabled(enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.serverRelayEnabled = enabled
}

// ServerRelayEnabled reports the current state of the switch above.
func (c *Coordinator) ServerRelayEnabled() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.serverRelayEnabled
}

// ReportServerLink records a real measured link from the server's own relay
// connection to peerID in room — not yet read back into Plan's synthetic
// server Report (see withServerReport, which still uses the generous
// serverUp/DownMbps placeholders); added now so wiring real measurement in
// later isn't a schema change.
func (c *Coordinator) ReportServerLink(room, peerID string, latencyMs, mbps float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	rs := c.room(room)
	if rs.serverLinks == nil {
		rs.serverLinks = map[string]LinkStat{}
	}
	rs.serverLinks[peerID] = LinkStat{Peer: peerID, LatencyMs: latencyMs, UpMbps: mbps}
}

func (c *Coordinator) room(roomID string) *roomState {
	rs, ok := c.rooms[roomID]
	if !ok {
		rs = &roomState{reports: make(map[string]*Report), lastSeen: make(map[string]time.Time)}
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
	rs.lastSeen[peerID] = time.Now()
	rs.generation++
}

// pruneStale evicts any peer that hasn't reported within staleAfter. Called
// with c.mu already held.
func (c *Coordinator) pruneStale(roomID string, rs *roomState) {
	cutoff := time.Now().Add(-staleAfter)
	for _, peerID := range rs.order {
		if seen, ok := rs.lastSeen[peerID]; ok && seen.Before(cutoff) {
			delete(rs.reports, peerID)
			delete(rs.lastSeen, peerID)
		}
	}
	kept := rs.order[:0]
	for _, id := range rs.order {
		if _, stillHere := rs.reports[id]; stillHere {
			kept = append(kept, id)
		}
	}
	rs.order = kept
	if len(rs.order) == 0 {
		delete(c.rooms, roomID)
	}
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

// SessionRow is one (room, peer) pair for admin/dashboard display — see
// Snapshot. Direction is derived from the same tree structure the optimizer
// already computed, not re-derived separately.
type SessionRow struct {
	Room          string
	PeerID        string
	UpMbps        float64
	DownMbps      float64
	Tier          string   // "" if no plan has been computed for this room yet
	Role          string   // "relay" if this peer forwards another peer's stream onward, else "direct"
	SendsTo       []string // peer ids this peer forwards (any source's) video to
	ReceivesFrom  []string // peer ids this peer receives video directly from
	LastSeenMsAgo int64
}

// RoomIDs returns every currently active room's id — read-only, for
// admin/dashboard use (see Snapshot).
func (c *Coordinator) RoomIDs() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, 0, len(c.rooms))
	for id := range c.rooms {
		out = append(out, id)
	}
	return out
}

// RelayEdge is one tree edge from a plan: Parent forwards Source's stream to
// Child (Parent == Source for a publisher's own direct sends).
type RelayEdge struct {
	Source string
	Parent string
	Child  string
}

// RelayEdges flattens every tree in a plan into its edges — the single
// source of truth for "who forwards whose stream to whom" that both
// DirectionsFromPlan and relaynode's own reconciliation build on, so they
// can never drift from what the plan actually says.
func RelayEdges(env *PlanEnvelope) []RelayEdge {
	var out []RelayEdge
	if env == nil {
		return out
	}
	for _, tree := range env.Plan.Trees {
		if tree.Source < 0 || tree.Source >= len(env.Order) {
			continue
		}
		srcID := env.Order[tree.Source]
		for nodeIdx, children := range tree.Children {
			if nodeIdx >= len(env.Order) {
				continue
			}
			nodeID := env.Order[nodeIdx]
			for _, childIdx := range children {
				if childIdx < 0 || childIdx >= len(env.Order) {
					continue
				}
				out = append(out, RelayEdge{Source: srcID, Parent: nodeID, Child: env.Order[childIdx]})
			}
		}
	}
	return out
}

// Directions is one peer's place in a room's relay tree: for each neighbor
// it exchanges video with, the source id(s) whose stream flows over that
// connection — e.g. SendsTo["7"] = ["3","9"] means this peer forwards peers
// 3's and 9's streams on to peer 7. Built once per plan by DirectionsFromPlan
// and shared by the admin Snapshot view and relaynode's own reconciliation.
type Directions struct {
	SendsTo      map[string][]string
	ReceivesFrom map[string][]string
}

// DirectionsFromPlan derives every peer's Directions from a plan's trees.
func DirectionsFromPlan(env *PlanEnvelope) map[string]Directions {
	out := map[string]Directions{}
	getOrInit := func(id string) Directions {
		d, ok := out[id]
		if !ok {
			d = Directions{SendsTo: map[string][]string{}, ReceivesFrom: map[string][]string{}}
			out[id] = d
		}
		return d
	}
	for _, e := range RelayEdges(env) {
		parent := getOrInit(e.Parent)
		parent.SendsTo[e.Child] = append(parent.SendsTo[e.Child], e.Source)
		child := getOrInit(e.Child)
		child.ReceivesFrom[e.Parent] = append(child.ReceivesFrom[e.Parent], e.Source)
	}
	return out
}

func mapKeysSorted(m map[string][]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Snapshot builds one SessionRow per (room, peer) across every active room —
// read-only; it reuses Plan (which already prunes stale peers) rather than
// re-deriving routing separately, so the admin view always matches what the
// room is actually being told to do.
func (c *Coordinator) Snapshot() []SessionRow {
	var out []SessionRow
	now := time.Now()
	for _, roomID := range c.RoomIDs() {
		env, _ := c.Plan(roomID)

		c.mu.Lock()
		rs, ok := c.rooms[roomID]
		if !ok {
			c.mu.Unlock()
			continue
		}
		order := append([]string(nil), rs.order...)
		reports := make(map[string]*Report, len(rs.reports))
		for id, r := range rs.reports {
			reports[id] = r
		}
		lastSeen := make(map[string]time.Time, len(rs.lastSeen))
		for id, t := range rs.lastSeen {
			lastSeen[id] = t
		}
		c.mu.Unlock()

		tierFor := map[string]string{}
		if env != nil {
			for id, t := range env.VideoTiers {
				tierFor[id] = fmt.Sprintf("%dx%d@%dkbps", t.Width, t.Height, t.BitrateKbps)
			}
		}
		dirs := DirectionsFromPlan(env)

		for _, peerID := range order {
			row := SessionRow{Room: roomID, PeerID: peerID, Tier: tierFor[peerID]}
			if rep := reports[peerID]; rep != nil {
				row.UpMbps = rep.Up
				row.DownMbps = rep.Down
			}
			d := dirs[peerID]
			row.SendsTo = mapKeysSorted(d.SendsTo)
			row.ReceivesFrom = mapKeysSorted(d.ReceivesFrom)
			row.Role = "direct"
			for _, sources := range d.SendsTo {
				for _, s := range sources {
					if s != peerID {
						row.Role = "relay"
					}
				}
			}
			if seen, ok := lastSeen[peerID]; ok {
				row.LastSeenMsAgo = now.Sub(seen).Milliseconds()
			}
			out = append(out, row)
		}
	}
	return out
}

// MatrixCell is one (row peer -> column peer) edge in a room's adjacency
// matrix — admin/dashboard use only, see RoomMatrix.
type MatrixCell struct {
	LatencyMs float64  `json:"latencyMs,omitempty"` // row's own self-reported measured latency to column, if known
	UpMbps    float64  `json:"upMbps,omitempty"`    // row's own self-reported measured uplink speed to column, if known
	Measured  bool     `json:"measured"`            // whether row has ever reported a link to column at all
	Sends     []string `json:"sends,omitempty"`     // source ids row forwards directly to column right now (own stream and/or relayed)
}

// RoomMatrix is a room's full peer-to-peer adjacency matrix — admin/
// dashboard use only. Peers includes ServerPeerID, with Retranslator set
// to it too, exactly when Plan has the server engaged in this room.
type RoomMatrix struct {
	Room         string         `json:"room"`
	Peers        []string       `json:"peers"`
	Retranslator string         `json:"retranslator,omitempty"`
	Matrix       [][]MatrixCell `json:"matrix"` // Matrix[i][j] is Peers[i] -> Peers[j]
}

// buildRoomMatrix is the pure computation behind RoomMatrix and Timeline:
// given a peer order, the reports behind it, and the plan built from them,
// it derives the full adjacency matrix. Pulled out so a historical moment
// (recorded reports + env, see historyPoint) and the live one can share
// identical cell logic.
func buildRoomMatrix(roomID string, peers []string, reports map[string]*Report, env *PlanEnvelope) *RoomMatrix {
	retranslator := ""
	for _, p := range peers {
		if p == ServerPeerID {
			retranslator = ServerPeerID
			break
		}
	}

	dirs := DirectionsFromPlan(env)

	matrix := make([][]MatrixCell, len(peers))
	for i, a := range peers {
		row := make([]MatrixCell, len(peers))
		for j, b := range peers {
			if a == b {
				continue
			}
			var cell MatrixCell
			if rep := reports[a]; rep != nil {
				for _, s := range rep.Stats {
					if s.Peer == b {
						cell.LatencyMs = s.LatencyMs
						cell.UpMbps = s.UpMbps
						cell.Measured = true
						break
					}
				}
			}
			if d, ok := dirs[a]; ok {
				cell.Sends = d.SendsTo[b]
			}
			row[j] = cell
		}
		matrix[i] = row
	}

	return &RoomMatrix{Room: roomID, Peers: peers, Retranslator: retranslator, Matrix: matrix}
}

// RoomMatrix builds roomID's current adjacency matrix: every peer in the
// room's current plan (or, if no plan is buildable yet, every peer that has
// reported) as both a row and column, each cell combining that row peer's
// own self-measured link to the column peer (from its last Report) with
// what it's actually forwarding there right now, per DirectionsFromPlan.
func (c *Coordinator) RoomMatrix(roomID string) *RoomMatrix {
	c.mu.Lock()
	rs, ok := c.rooms[roomID]
	if !ok {
		c.mu.Unlock()
		return nil
	}
	order := append([]string(nil), rs.order...)
	reports := make(map[string]*Report, len(rs.reports))
	for id, r := range rs.reports {
		reports[id] = r
	}
	c.mu.Unlock()

	env, _ := c.Plan(roomID)
	peers := order
	if env != nil {
		peers = env.Order
	}

	return buildRoomMatrix(roomID, peers, reports, env)
}

// appendHistory records one Plan() outcome into roomID's bounded history,
// deduplicated by generation so repeated/polling Plan() calls (e.g. the
// admin dashboard's own replan via Snapshot) between real Report() calls
// don't pile up near-identical points — called from Plan()'s defer.
func (c *Coordinator) appendHistory(roomID string, env *PlanEnvelope, reports map[string]*Report) {
	c.mu.Lock()
	defer c.mu.Unlock()
	rs, ok := c.rooms[roomID]
	if !ok || rs.generation == rs.lastHistoryGen {
		return
	}
	rs.lastHistoryGen = rs.generation
	rs.history = append(rs.history, historyPoint{Time: time.Now(), Env: env, Reports: reports})
	if len(rs.history) > maxHistoryPoints {
		rs.history = rs.history[len(rs.history)-maxHistoryPoints:]
	}
}

// TimelinePoint is one recorded moment in a room's history — admin/
// dashboard use only, see Timeline.
type TimelinePoint struct {
	Time   time.Time   `json:"time"`
	Matrix *RoomMatrix `json:"matrix"`
}

// Timeline returns roomID's recent plan history (oldest first, bounded to
// maxHistoryPoints), each point's adjacency matrix rebuilt from what was
// actually reported and planned at that moment — lets an admin scrub back
// through how a room's routing evolved (see meshsessions' timeline slider).
func (c *Coordinator) Timeline(roomID string) []TimelinePoint {
	c.mu.Lock()
	rs, ok := c.rooms[roomID]
	if !ok {
		c.mu.Unlock()
		return nil
	}
	points := append([]historyPoint(nil), rs.history...)
	c.mu.Unlock()

	out := make([]TimelinePoint, 0, len(points))
	for _, p := range points {
		peers := []string(nil)
		if p.Env != nil {
			peers = p.Env.Order
		}
		out = append(out, TimelinePoint{Time: p.Time, Matrix: buildRoomMatrix(roomID, peers, p.Reports, p.Env)})
	}
	return out
}

// planHooks fire after every Plan() call with the room id and the resulting
// envelope (nil if not yet buildable) — same callback-injection idiom as
// RegisterSignalQueuedHook, used so relaynode's own reconciliation runs
// automatically after every planning pass, regardless of which module's
// HTTP handler triggered it (p2prendezvous's calls.go, papers' generic
// ReportLink/GetPlan, or Snapshot's own admin-view replan), without this
// package importing relaynode (which would cycle back here).
var planHooks []func(room string, env *PlanEnvelope)

func RegisterPlanHook(fn func(room string, env *PlanEnvelope)) {
	planHooks = append(planHooks, fn)
}

// Plan builds a relay plan for the room from the reports gathered so far. It
// returns (nil, waiting) when fewer than two peers are present or a present
// peer has not reported yet; waiting lists the peers still missing.
//
// When the server's own relay-node switch is on (SetServerRelayEnabled,
// default on), each call also decides whether the server itself should join
// this room as an extra relay peer (ServerPeerID): the room's real
// participants are planned alone first, and the server is only added when
// that plan would run hot — at or over engageServerUtilThreshold
// utilization, or the room already has engageServerPeerCount+ peers — with
// hysteresis (disengageServerUtilThreshold, lower) so a room near the
// boundary doesn't flap the server in and out every cycle. relaynode's own
// reconciliation reads the result (ServerPeerID present in env.Order or
// not) to know whether it should be connected to this room at all.
func (c *Coordinator) Plan(roomID string) (env *PlanEnvelope, waiting []string) {
	var histReports map[string]*Report
	defer func() {
		if env != nil {
			c.appendHistory(roomID, env, histReports)
		}
		for _, fn := range planHooks {
			fn(roomID, env)
		}
	}()

	c.mu.Lock()
	rs, ok := c.rooms[roomID]
	if !ok {
		c.mu.Unlock()
		return nil, nil
	}
	c.pruneStale(roomID, rs)
	if _, stillExists := c.rooms[roomID]; !stillExists {
		c.mu.Unlock()
		return nil, nil
	}
	order := append([]string(nil), rs.order...)
	reports := make(map[string]*Report, len(rs.reports))
	for id, r := range rs.reports {
		reports[id] = r
	}
	histReports = reports
	serverRelayEnabled := c.serverRelayEnabled
	wasEngaged := rs.serverEngaged
	c.mu.Unlock()

	humanEnv, waiting := BuildPlanFor(order, reports)
	if !serverRelayEnabled || humanEnv == nil {
		c.setServerEngaged(roomID, false)
		return humanEnv, waiting
	}

	threshold := engageServerUtilThreshold
	if wasEngaged {
		threshold = disengageServerUtilThreshold
	}
	res := humanEnv.Plan.Result
	humanRunsHot := res.MaxUpUtil >= threshold || res.MaxDownUtil >= threshold || res.MaxRelayUtil >= threshold
	needsServer := humanRunsHot || len(order) >= engageServerPeerCount

	if !needsServer {
		c.setServerEngaged(roomID, false)
		return humanEnv, waiting
	}

	serverOrder, serverReports := withServerReport(order, reports)
	withServerEnv, serverWaiting := BuildPlanFor(serverOrder, serverReports)
	if withServerEnv == nil {
		c.setServerEngaged(roomID, false)
		return humanEnv, waiting
	}
	c.setServerEngaged(roomID, true)
	return withServerEnv, serverWaiting
}

func (c *Coordinator) setServerEngaged(roomID string, engaged bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if rs, ok := c.rooms[roomID]; ok {
		rs.serverEngaged = engaged
	}
}

// withServerReport appends the server's synthetic peer id/report to a
// room's real order/reports, for the "server joins as relay" branch of
// Plan. Its LinkStat to every real peer is an assumed-good placeholder
// (same bootstrap-before-first-probe idea a human's own CallProber already
// relies on) until ReportServerLink has real numbers.
func withServerReport(order []string, reports map[string]*Report) ([]string, map[string]*Report) {
	stats := make([]LinkStat, 0, len(order))
	for _, id := range order {
		stats = append(stats, LinkStat{Peer: id, LatencyMs: 20, UpMbps: serverUpMbps})
	}
	newOrder := append(append([]string(nil), order...), ServerPeerID)
	newReports := make(map[string]*Report, len(reports)+1)
	for id, r := range reports {
		newReports[id] = r
	}
	newReports[ServerPeerID] = &Report{
		Up:       serverUpMbps,
		Down:     serverDownMbps,
		Stats:    stats,
		Platform: "server",
	}
	return newOrder, newReports
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
	// RelayCap is left nil by blankInput (optimizer.Input.Normalize fills it with
	// unlimited later) — pre-fill it here so a mobile platform's entry below has
	// somewhere to land; everyone else keeps the historical unlimited default.
	in.RelayCap = make([]float64, n)
	for i := range in.RelayCap {
		in.RelayCap[i] = math.Inf(1)
	}
	for i, id := range order {
		rep := reports[id]
		in.Up[i] = orDefault(rep.Up, 10)
		in.Down[i] = orDefault(rep.Down, 50)
		if rep.Platform != "" && rep.Platform != "web" {
			cap := mobileRelayCapMbps
			if v, ok := platformRelayCapMbps[rep.Platform]; ok {
				cap = v
			}
			in.RelayCap[i] = cap
		}
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
	// The server (ServerPeerID) is never a publisher or a watcher — only a
	// relay — so it must never get a demand row or column, or the optimizer
	// plans a phantom "server broadcasts its own stream" tree that no real
	// media will ever exist for (relaynode only ever forwards a source it
	// actually received a track for).
	setDemand := func() {
		for i := 0; i < n; i++ {
			if order[i] == ServerPeerID {
				continue
			}
			mbps := float64(videoTiers[tierIdx[i]].BitrateKbps) / 1000.0
			for j := 0; j < n; j++ {
				if i != j && order[j] != ServerPeerID {
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
