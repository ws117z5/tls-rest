package optimizer

import (
	"container/heap"
	"math"
)

// Input is the measured description of the overlay. Matrices are N x N indexed
// [from][to]. Rates are Mbit/s, latencies ms.
type Input struct {
	N   int
	Lat [][]float64 // one-way latency of the direct link i->j (ms)
	BW  [][]float64 // network-path capacity of the direct link i->j (Mbit/s)

	// Demand[i][j] is the sustained rate node i wants to deliver to node j.
	Demand [][]float64

	// Per-node shared pipes. These are the big asymmetric constraints for real
	// clients: everything a node sends (its own traffic AND anything it relays)
	// shares Up[v]; everything it receives (its own AND relayed) shares Down[v].
	// nil => treated as unlimited (falls back to pure per-link behaviour).
	Up   []float64 // uplink capacity per node (Mbit/s)
	Down []float64 // downlink capacity per node (Mbit/s)

	// Optional extra forwarding cap charged ONLY to pass-through traffic
	// (e.g. a CPU / packet-rate limit distinct from bandwidth). nil => unlimited.
	RelayCap  []float64
	ProcDelay []float64 // extra latency when a node relays (ms); nil => 0

	// Tuning.
	HopPenalty   float64 // ms added per overlay hop -> discourages needless relaying
	MaxRelayHops int     // max relays on a path (1 = at most one intermediary)
	PipeLatency  float64 // nominal host serialization latency on up/down edges (ms)
}

func infSlice(n int) []float64 {
	s := make([]float64, n)
	for i := range s {
		s[i] = math.Inf(1)
	}
	return s
}

// Normalize fills defaults.
func (in *Input) Normalize() {
	if in.MaxRelayHops == 0 {
		in.MaxRelayHops = 1
	}
	if in.HopPenalty == 0 {
		in.HopPenalty = 0.5
	}
	if in.PipeLatency == 0 {
		in.PipeLatency = 0.2 // small: only bites as a pipe nears saturation
	}
	if in.Up == nil {
		in.Up = infSlice(in.N)
	}
	if in.Down == nil {
		in.Down = infSlice(in.N)
	}
	if in.RelayCap == nil {
		in.RelayCap = infSlice(in.N)
	}
	if in.ProcDelay == nil {
		in.ProcDelay = make([]float64, in.N)
	}
}

// ---------------------------------------------------------------------------
// Split graph.
//
// Each node v is split into FOUR vertices so that both shared pipes and the
// relay budget are ordinary capacitated edges:
//
//	vA  = 4v     arrival hub (raw bytes in from the network)
//	vIn = 4v+1   after the downlink   (also the commodity SINK for dest v)
//	vOut= 4v+2   before the uplink    (also the commodity SOURCE for origin v)
//	vB  = 4v+3   departure hub (raw bytes out to the network)
//
// Edges per node:
//	downlink   vA  -> vIn   cap = Down[v]      (all arriving traffic)
//	uplink     vOut-> vB    cap = Up[v]        (all departing traffic)
//	relay      vIn -> vOut  cap = RelayCap[v]  (pass-through only; +ProcDelay)
//
// Physical link a->b:  vB(a) -> vA(b)  cap = BW[a][b], latency Lat[a][b].
//
// A flow s->d routes from vOut(s) to vIn(d):
//	direct : vOut(s)->vB(s)[up] -> vA(d)[link] -> vIn(d)[down]
//	relay r: ...->vA(r)[link]->vIn(r)[down]->vOut(r)[relay]->vB(r)[up]->vA(d)[link]->vIn(d)[down]
// so a relay pays r's downlink AND uplink, exactly like a real forwarder, and a
// source pays only its uplink, a destination only its downlink.
// ---------------------------------------------------------------------------

func vA(v int) int   { return 4 * v }
func vIn(v int) int  { return 4*v + 1 }
func vOut(v int) int { return 4*v + 2 }
func vB(v int) int   { return 4*v + 3 }

type edge struct {
	from, to int
	t0       float64 // free-flow latency (ms)
	cap      float64 // capacity (Mbit/s); +Inf means uncapacitated
	internal bool    // true only for the relay pass-through edge (counts as a hop)
	flow     float64
}

type graph struct {
	V   int
	adj [][]int
	e   []edge
	in  *Input
}

func buildGraph(in *Input) *graph {
	g := &graph{V: 4 * in.N, in: in}
	g.adj = make([][]int, g.V)
	add := func(from, to int, t0, cap float64, internal bool) {
		g.adj[from] = append(g.adj[from], len(g.e))
		g.e = append(g.e, edge{from: from, to: to, t0: t0, cap: cap, internal: internal})
	}
	for v := 0; v < in.N; v++ {
		add(vA(v), vIn(v), in.PipeLatency, in.Down[v], false)       // downlink
		add(vOut(v), vB(v), in.PipeLatency, in.Up[v], false)        // uplink
		add(vIn(v), vOut(v), in.ProcDelay[v], in.RelayCap[v], true) // relay pass-through
	}
	for a := 0; a < in.N; a++ {
		for b := 0; b < in.N; b++ {
			if a == b || in.BW[a][b] <= 0 {
				continue
			}
			add(vB(a), vA(b), in.Lat[a][b]+in.HopPenalty, in.BW[a][b], false)
		}
	}
	return g
}

// cost: M/M/1 queueing barrier t0/(1-u). Equals t0 idle, -> Inf at capacity, so
// a hard bitrate limit (link OR shared pipe) is actually respected.
func (g *graph) cost(e *edge, x float64) float64 {
	if math.IsInf(e.cap, 1) {
		return e.t0
	}
	if e.cap <= 0 {
		return math.Inf(1)
	}
	u := x / e.cap
	if u >= capCeil {
		u = capCeil
	}
	return e.t0 / (1 - u)
}

// marginal = d/dx[x*cost(x)] = t0/(1-u)^2 : the extra system latency one more
// unit of flow imposes. Used by the system-optimal shortest-path oracle.
func (g *graph) marginal(e *edge, x float64) float64 {
	if math.IsInf(e.cap, 1) {
		return e.t0
	}
	if e.cap <= 0 {
		return math.Inf(1)
	}
	u := x / e.cap
	if u >= capCeil {
		u = capCeil
	}
	d := 1 - u
	return e.t0 / (d * d)
}

const capCeil = 0.999

func (g *graph) totalTime() float64 {
	var t float64
	for i := range g.e {
		e := &g.e[i]
		t += e.flow * g.cost(e, e.flow)
	}
	return t
}

// ------------------------- hop-limited Dijkstra -------------------------

type state struct{ vertex, relays int }
type pqItem struct {
	st   state
	dist float64
	idx  int
}
type pq []*pqItem

func (p pq) Len() int            { return len(p) }
func (p pq) Less(i, j int) bool  { return p[i].dist < p[j].dist }
func (p pq) Swap(i, j int)       { p[i], p[j] = p[j], p[i]; p[i].idx = i; p[j].idx = j }
func (p *pq) Push(x interface{}) { *p = append(*p, x.(*pqItem)) }
func (p *pq) Pop() interface{} {
	old := *p
	n := len(old)
	it := old[n-1]
	*p = old[:n-1]
	return it
}

// shortestPath: edge indices from vOut(src) to vIn(dst) under weight(), using at
// most maxRelays relay edges. nil if unreachable.
func (g *graph) shortestPath(src, dst, maxRelays int, weight func(ei int) float64) []int {
	start := state{vertex: vOut(src)}
	goal := vIn(dst)
	dist := map[state]float64{start: 0}
	prevEdge := map[state]int{}
	prevState := map[state]state{}
	h := &pq{{st: start}}
	heap.Init(h)
	for h.Len() > 0 {
		cur := heap.Pop(h).(*pqItem)
		if cur.dist > dist[cur.st] {
			continue
		}
		if cur.st.vertex == goal {
			var path []int
			s := cur.st
			for s != start {
				ei := prevEdge[s]
				path = append([]int{ei}, path...)
				s = prevState[s]
			}
			return path
		}
		for _, ei := range g.adj[cur.st.vertex] {
			e := &g.e[ei]
			nr := cur.st.relays
			if e.internal {
				if nr+1 > maxRelays {
					continue
				}
				nr++
			}
			w := weight(ei)
			if math.IsInf(w, 1) {
				continue
			}
			ns := state{vertex: e.to, relays: nr}
			nd := cur.dist + w
			if old, ok := dist[ns]; !ok || nd < old {
				dist[ns] = nd
				prevEdge[ns] = ei
				prevState[ns] = cur.st
				heap.Push(h, &pqItem{st: ns, dist: nd})
			}
		}
	}
	return nil
}
