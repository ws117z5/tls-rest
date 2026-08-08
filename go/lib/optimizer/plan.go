package optimizer

import "sort"

// Tree is a distribution tree for one media source over the balanced overlay.
// Every other node receives the source exactly once (from Parent) and forwards
// it to Children — so a node is a relay for this source iff it has children it
// is not the source of.
type Tree struct {
	Source   int     `json:"source"`
	Parent   []int   `json:"parent"`   // Parent[v] = upstream node, -1 for source/unreachable
	Children [][]int `json:"children"` // Children[v] = nodes v forwards this source to
	HopCount []int   `json:"hopCount"` // overlay hops from source to v (source=0)
}

// Plan is what the coordinator ships to clients: one distribution tree per
// source, plus the aggregate quality metrics.
type Plan struct {
	N      int     `json:"n"`
	Trees  []Tree  `json:"trees"`
	Result *Result `json:"result"`
}

// BuildPlan solves the balanced assignment, then derives a single distribution
// tree per source from the converged (load-aware) marginal costs. Trees are the
// right structure for one-copy-per-link media fan-out: unlike raw per-(s,d)
// paths they guarantee each node has one parent per source, so relayed streams
// are forwarded exactly once.
func BuildPlan(in *Input) *Plan {
	g, coms, iter, gap := solve(in)
	res := buildResult(g, in, coms, iter, gap)

	weight := func(ei int) float64 {
		e := &g.e[ei]
		return g.marginal(e, e.flow)
	}

	plan := &Plan{N: in.N, Result: res}
	for s := 0; s < in.N; s++ {
		plan.Trees = append(plan.Trees, buildTree(g, in, s, weight))
	}
	return plan
}

// buildTree constructs a valid tree from source s by taking the balanced
// shortest path to every destination and stitching them nearest-first: a node's
// parent is fixed by the closest destination whose path passes through it, which
// makes the union of paths a tree (no node ends up with two parents).
func buildTree(g *graph, in *Input, s int, weight func(int) float64) Tree {
	t := Tree{Source: s}
	t.Parent = make([]int, in.N)
	t.Children = make([][]int, in.N)
	t.HopCount = make([]int, in.N)
	for i := range t.Parent {
		t.Parent[i] = -1
		t.HopCount[i] = -1
	}
	t.HopCount[s] = 0

	// Shortest path (as node hops) to every destination, with its cost.
	type dpath struct {
		hops []int
		cost float64
	}
	paths := make([]dpath, 0, in.N)
	for d := 0; d < in.N; d++ {
		if d == s {
			continue
		}
		edges := g.shortestPath(s, d, in.MaxRelayHops, weight)
		if edges == nil {
			continue
		}
		var c float64
		for _, ei := range edges {
			c += weight(ei)
		}
		paths = append(paths, dpath{hops: edgesToHops(g, edges), cost: c})
	}
	// Nearest-first so parents are assigned by the cheapest path through a node.
	sort.Slice(paths, func(i, j int) bool { return paths[i].cost < paths[j].cost })

	for _, p := range paths {
		for k := 1; k < len(p.hops); k++ {
			a, b := p.hops[k-1], p.hops[k]
			if t.Parent[b] == -1 && b != s {
				t.Parent[b] = a
				t.Children[a] = append(t.Children[a], b)
				if t.HopCount[a] >= 0 {
					t.HopCount[b] = t.HopCount[a] + 1
				}
			}
		}
	}
	return t
}
