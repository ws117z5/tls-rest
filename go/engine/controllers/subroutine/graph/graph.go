package graph

import (
	"crypto/sha256"
	"fmt"
	"os"
)

type Graph struct {
	Nodes  int
	Fanout int
	Edges  map[int][]int
}

// Generate deterministic neighbors using SHA256
func getNeighbors(nodeID, fanout, total int) []int {
	neighbors := make(map[int]struct{})
	i := 0
	for len(neighbors) < fanout {
		key := fmt.Sprintf("%d-%d", nodeID, i)
		hash := sha256.Sum256([]byte(key))
		num := int(hash[0])<<8 + int(hash[1]) // use first 2 bytes for simplicity
		neighbor := num % total
		if neighbor != nodeID {
			neighbors[neighbor] = struct{}{}
		}
		i++
	}
	result := make([]int, 0, fanout)
	for k := range neighbors {
		result = append(result, k)
	}
	return result
}

// Generate graph structure
func NewGraph(nodes, fanout int) *Graph {
	g := &Graph{
		Nodes:  nodes,
		Fanout: fanout,
		Edges:  make(map[int][]int),
	}
	for i := 0; i < nodes; i++ {
		g.Edges[i] = getNeighbors(i, fanout, nodes)
	}
	return g
}

// Output .dot file for Graphviz visualization
func (g *Graph) WriteDOT(filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprintln(f, "digraph G {")
	for from, tos := range g.Edges {
		for _, to := range tos {
			fmt.Fprintf(f, "  %d -> %d;\n", from, to)
		}
	}
	fmt.Fprintln(f, "}")
	return nil
}
