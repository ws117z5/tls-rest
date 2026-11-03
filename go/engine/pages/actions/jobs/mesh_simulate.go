// Synthetic mesh-session traffic, backing the "mesh_simulate" Action — lets
// the meshsessions module's adjacency-matrix view be exercised without a
// real WebRTC client running.
package jobs

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	actionsctl "tls-rest/go/engine/controllers/actions"
	"tls-rest/go/engine/controllers/mesh"
)

// meshSimRoom is fixed (not per-run) so re-running the action restarts the
// same demo session rather than piling up abandoned rooms; meshSimMu/
// meshSimCancel let a re-run stop the previous run's goroutine before
// starting a fresh one.
const meshSimRoom = "sim:demo"

var meshSimPeers = []string{"sim-alice", "sim-bob", "sim-carol", "sim-dave"}

var (
	meshSimMu     sync.Mutex
	meshSimCancel func()
)

// runMeshSimulation seeds meshSimRoom with synthetic peers and re-reports
// mildly randomized link stats every 2s for up to 60s. Left alone, it just
// stops reporting and the Coordinator's own staleness pruning (see
// coordinator.go) evicts it 20s later.
func runMeshSimulation(ctx context.Context) {
	coord := mesh.DefaultCoordinator()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	deadline := time.Now().Add(60 * time.Second)

	report := func() {
		for _, p := range meshSimPeers {
			stats := make([]mesh.LinkStat, 0, len(meshSimPeers)-1)
			for _, q := range meshSimPeers {
				if q == p {
					continue
				}
				stats = append(stats, mesh.LinkStat{
					Peer:      q,
					LatencyMs: 15 + rand.Float64()*70,
					UpMbps:    3 + rand.Float64()*12,
				})
			}
			coord.Report(meshSimRoom, p, &mesh.Report{
				Up:       3 + rand.Float64()*12,
				Down:     10 + rand.Float64()*20,
				Stats:    stats,
				Platform: "sim",
			})
		}
	}

	report()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if time.Now().After(deadline) {
				return
			}
			report()
		}
	}
}

// startMeshSimulation is the "mesh_simulate" Action's Run: cancels any prior
// run, starts a fresh one in the background, and returns immediately (the
// Action.Run contract expects a quick result, not a 60s block).
func startMeshSimulation() (string, error) {
	meshSimMu.Lock()
	if meshSimCancel != nil {
		meshSimCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	meshSimCancel = cancel
	meshSimMu.Unlock()

	go runMeshSimulation(ctx)

	return fmt.Sprintf("started mesh session simulation in room %q (%d peers, ~60s)", meshSimRoom, len(meshSimPeers)), nil
}

func init() {
	actionsctl.Register(&actionsctl.Action{
		ID:          "mesh_simulate",
		Name:        "Simulate mesh session",
		Description: "Seeds a synthetic WebRTC mesh call (room \"sim:demo\", 4 peers) with randomized link stats every 2s for ~60s, so the Mesh Sessions module has live data to show without a real call running.",
		Run:         startMeshSimulation,
	})
}
