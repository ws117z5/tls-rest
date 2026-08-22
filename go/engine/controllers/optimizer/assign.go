package optimizer

import (
	"fmt"
	"math"
	"os"
	"sort"
)

var debug = os.Getenv("MESHOPT_DEBUG") != ""

// Commodity is one origin->destination demand.
type commodity struct {
	s, d   int
	demand float64
	// path flows keyed by a canonical signature of the edge list.
	paths map[string]*pathFlow
}

type pathFlow struct {
	edges []int
	flow  float64
}

// aonTarget is one commodity's all-or-nothing shortest path for an iteration.
type aonTarget struct {
	c     *commodity
	edges []int
}

func pathKey(edges []int) string {
	// edge indices already identify the exact path in this static graph.
	return fmt.Sprint(edges)
}

// Result is what the optimizer hands back.
type Result struct {
	Routes     []Route   // one entry per (s,d) with positive demand
	EdgeLoad   []float64 // final flow on every graph edge (indexed like graph.e)
	Iterations int
	Gap        float64 // final relative Frank-Wolfe gap
	// Aggregate quality metrics.
	MeanLatency       float64 // demand-weighted delivery latency (ms)
	MaxRelayUtil      float64 // most loaded relay forwarding budget (fraction)
	MaxUpUtil         float64 // most loaded uplink pipe (fraction)
	MaxDownUtil       float64 // most loaded downlink pipe (fraction)
	DirectMeanLatency float64 // same metric if everyone went direct (baseline)
}

// Route is the recommended (possibly multi-path) routing for one flow.
type Route struct {
	Src, Dst int
	Paths    []PathShare
}

// PathShare is one path carrying a fraction of a flow.
type PathShare struct {
	Hops     []int   // node ids: [src, ...relays..., dst]
	Fraction float64 // share of this flow's demand (0..1)
	Latency  float64 // free-flow latency of this path (ms)
}

// solve runs Frank-Wolfe to convergence and returns the converged graph plus
// per-commodity path flows.
func solve(in *Input) (*graph, []*commodity, int, float64) {
	in.Normalize()
	g := buildGraph(in)

	// Build commodity list.
	var coms []*commodity
	for s := 0; s < in.N; s++ {
		for d := 0; d < in.N; d++ {
			if s == d || in.Demand[s][d] <= 0 {
				continue
			}
			coms = append(coms, &commodity{s: s, d: d, demand: in.Demand[s][d],
				paths: map[string]*pathFlow{}})
		}
	}

	// Weight oracle uses *marginal* cost -> drives toward the system optimum.
	marginalWeight := func(ei int) float64 {
		e := &g.e[ei]
		return g.marginal(e, e.flow)
	}
	recomputeEdgeFlows := func() {
		for i := range g.e {
			g.e[i].flow = 0
		}
		for _, c := range coms {
			for _, pf := range c.paths {
				for _, ei := range pf.edges {
					g.e[ei].flow += pf.flow
				}
			}
		}
	}

	// Iteration 0: all-or-nothing on free-flow marginal costs.
	for _, c := range coms {
		p := g.shortestPath(c.s, c.d, in.MaxRelayHops, marginalWeight)
		if p == nil {
			continue // isolated pair; skipped
		}
		c.paths[pathKey(p)] = &pathFlow{edges: p, flow: c.demand}
	}
	recomputeEdgeFlows()

	const maxIter = 300
	const tol = 1.5e-3 // stop within ~0.15% of the optimum (FW tail is slow)
	var iter int
	var gap float64
	for iter = 1; iter <= maxIter; iter++ {
		// All-or-nothing target y under current marginal costs.
		targets := make([]aonTarget, 0, len(coms))
		var num, den float64
		for _, c := range coms {
			p := g.shortestPath(c.s, c.d, in.MaxRelayHops, marginalWeight)
			if p == nil {
				continue
			}
			targets = append(targets, aonTarget{c: c, edges: p})
			// gap numerator: current marginal cost of served flow minus best.
			var cur, best float64
			for _, pf := range c.paths {
				pc := pathMarginal(g, pf.edges)
				cur += pf.flow * pc
			}
			best = c.demand * pathMarginal(g, p)
			num += cur - best
			den += cur
		}
		if den > 0 {
			gap = num / den
		}
		if gap < tol && iter > 1 {
			break
		}

		// Frank-Wolfe step size via line search on the true objective.
		lambda := lineSearch(g, coms, targets)

		if debug {
			var maxU float64
			for i := range g.e {
				e := &g.e[i]
				if e.internal && !math.IsInf(e.cap, 1) && e.cap > 0 {
					if u := e.flow / e.cap; u > maxU {
						maxU = u
					}
				}
			}
			fmt.Fprintf(os.Stderr, "iter %3d  gap=%.3e  lambda=%.4f  T=%.1f  maxRelayUtil=%.0f%%\n",
				iter, gap, lambda, g.totalTime(), 100*maxU)
		}

		// Move each commodity's path flows toward its all-or-nothing target.
		for i := range targets {
			c := targets[i].c
			for _, pf := range c.paths {
				pf.flow *= (1 - lambda)
			}
			k := pathKey(targets[i].edges)
			if pf, ok := c.paths[k]; ok {
				pf.flow += lambda * c.demand
			} else {
				c.paths[k] = &pathFlow{edges: targets[i].edges, flow: lambda * c.demand}
			}
			// prune numerically-dead paths
			for key, pf := range c.paths {
				if pf.flow < 1e-9 {
					delete(c.paths, key)
				}
			}
		}
		recomputeEdgeFlows()
	}

	return g, coms, iter, gap
}

// Optimize runs the pipeline and returns the balanced-mesh statistics.
func Optimize(in *Input) *Result {
	g, coms, iter, gap := solve(in)
	return buildResult(g, in, coms, iter, gap)
}

// pathMarginal sums marginal edge costs along a path at current flows.
func pathMarginal(g *graph, edges []int) float64 {
	var s float64
	for _, ei := range edges {
		e := &g.e[ei]
		s += g.marginal(e, e.flow)
	}
	return s
}

// lineSearch finds lambda in [0,1] minimising the system objective along the
// segment x + lambda*(y - x), using bisection on the objective's derivative.
func lineSearch(g *graph, coms []*commodity, targets []aonTarget) float64 {
	// Precompute, per edge, current flow x_e and target flow y_e.
	x := make([]float64, len(g.e))
	for i := range g.e {
		x[i] = g.e[i].flow
	}
	y := make([]float64, len(g.e))
	// y = all-or-nothing edge flows for the target paths.
	served := map[*commodity]bool{}
	for _, t := range targets {
		served[t.c] = true
		for _, ei := range t.edges {
			y[ei] += t.c.demand
		}
	}
	// Commodities without a target keep their current split contribution in y.
	for _, c := range coms {
		if served[c] {
			continue
		}
		for _, pf := range c.paths {
			for _, ei := range pf.edges {
				y[ei] += pf.flow
			}
		}
	}

	dir := make([]float64, len(g.e))
	for i := range g.e {
		dir[i] = y[i] - x[i]
	}
	// derivative of objective wrt lambda: sum_e dir_e * marginal_e(x_e + lambda*dir_e)
	deriv := func(l float64) float64 {
		var s float64
		for i := range g.e {
			if dir[i] == 0 {
				continue
			}
			f := x[i] + l*dir[i]
			s += dir[i] * g.marginal(&g.e[i], f)
		}
		return s
	}
	if deriv(0) >= 0 {
		return 0
	}
	if deriv(1) <= 0 {
		return 1
	}
	lo, hi := 0.0, 1.0
	for k := 0; k < 40; k++ {
		mid := (lo + hi) / 2
		if deriv(mid) > 0 {
			hi = mid
		} else {
			lo = mid
		}
	}
	return (lo + hi) / 2
}

func buildResult(g *graph, in *Input, coms []*commodity, iter int, gap float64) *Result {
	res := &Result{Iterations: iter, Gap: gap}
	res.EdgeLoad = make([]float64, len(g.e))
	for i := range g.e {
		res.EdgeLoad[i] = g.e[i].flow
	}

	var wLat, wDirect, totDemand float64
	for _, c := range coms {
		r := Route{Src: c.s, Dst: c.d}
		// order paths by descending share for readable output
		var shares []PathShare
		for _, pf := range c.paths {
			hops := edgesToHops(g, pf.edges)
			lat := freeFlowLatency(g, pf.edges)
			shares = append(shares, PathShare{
				Hops:     hops,
				Fraction: pf.flow / c.demand,
				Latency:  lat,
			})
			wLat += pf.flow * lat
		}
		sort.Slice(shares, func(i, j int) bool { return shares[i].Fraction > shares[j].Fraction })
		r.Paths = shares
		res.Routes = append(res.Routes, r)

		// baseline: everyone direct
		wDirect += c.demand * (in.Lat[c.s][c.d] + in.HopPenalty)
		totDemand += c.demand
	}
	sort.Slice(res.Routes, func(i, j int) bool {
		if res.Routes[i].Src != res.Routes[j].Src {
			return res.Routes[i].Src < res.Routes[j].Src
		}
		return res.Routes[i].Dst < res.Routes[j].Dst
	})
	if totDemand > 0 {
		res.MeanLatency = wLat / totDemand
		res.DirectMeanLatency = wDirect / totDemand
	}

	// utilisation: relay pass-through edges are internal; uplink is vOut->vB,
	// downlink is vA->vIn. Identify by endpoints in the 4-per-node layout.
	for i := range g.e {
		e := &g.e[i]
		if math.IsInf(e.cap, 1) || e.cap <= 0 {
			continue
		}
		u := e.flow / e.cap
		switch {
		case e.internal:
			if u > res.MaxRelayUtil {
				res.MaxRelayUtil = u
			}
		case e.from%4 == 2 && e.to%4 == 3: // vOut -> vB : uplink
			if u > res.MaxUpUtil {
				res.MaxUpUtil = u
			}
		case e.from%4 == 0 && e.to%4 == 1: // vA -> vIn : downlink
			if u > res.MaxDownUtil {
				res.MaxDownUtil = u
			}
		}
	}
	return res
}

// edgesToHops converts a path of split-graph edges into node ids. Only physical
// links cross node boundaries (from/4 != to/4); intra-node pipe/relay edges are
// skipped.
func edgesToHops(g *graph, edges []int) []int {
	var hops []int
	for _, ei := range edges {
		e := &g.e[ei]
		from := e.from / 4
		to := e.to / 4
		if from == to {
			continue // uplink / downlink / relay edge, stays within a node
		}
		if len(hops) == 0 {
			hops = append(hops, from)
		}
		hops = append(hops, to)
	}
	return hops
}

func freeFlowLatency(g *graph, edges []int) float64 {
	var s float64
	for _, ei := range edges {
		s += g.e[ei].t0
	}
	return s
}
