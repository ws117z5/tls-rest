package papers

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"tls-rest/go/engine/controllers/mesh"

	"github.com/gorilla/mux"
)

// meshCoordinator holds per-room link reports and turns them into balanced
// relay plans (the optimizer's balancer + resolver), including which video
// tier (resolution/bitrate) the room can currently sustain.
var meshCoordinator = mesh.NewCoordinator()

// ReportLink records a peer's measured links and returns the current plan (or a
// "waiting" status while other peers still need to report).
func ReportLink(w http.ResponseWriter, r *http.Request) {
	roomID := mux.Vars(r)["roomId"]

	b, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var req ReportRequest
	if err := json.Unmarshal(b, &req); err != nil {
		http.Error(w, "invalid report: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Peer == "" {
		req.Peer = r.URL.Query().Get("peer")
	}
	if req.Peer == "" {
		http.Error(w, "missing peer id", http.StatusBadRequest)
		return
	}

	log.Printf("[papers/mesh] room=%s report from %s: up=%.1f down=%.1f stats=%d link(s)",
		roomID, req.Peer, req.Up, req.Down, len(req.Stats))

	meshCoordinator.Report(roomID, req.Peer, &mesh.Report{
		Up:    req.Up,
		Down:  req.Down,
		Stats: req.Stats,
	})

	env, waiting := meshCoordinator.Plan(roomID)
	logPlanResult(roomID, env, waiting)
	writeMeshPlan(w, env, waiting)
}

// GetPlan returns the current relay plan for a room, or a waiting status.
func GetPlan(w http.ResponseWriter, r *http.Request) {
	roomID := mux.Vars(r)["roomId"]
	env, waiting := meshCoordinator.Plan(roomID)
	writeMeshPlan(w, env, waiting)
}

// logPlanResult logs the balancer's outcome whenever a report causes a plan to
// be (re)built — meant to stay on through a beta so relay/latency behavior
// under real networks can be reviewed from the server logs, not just guessed
// at. Deliberately only called from ReportLink (once per new report), not from
// the polling GetPlan path, or every client's periodic /plan refetch would
// flood the log with recomputations of an unchanged plan.
func logPlanResult(roomID string, env *mesh.PlanEnvelope, waiting []string) {
	if env != nil {
		res := env.Plan.Result
		log.Printf(
			"[papers/mesh] room=%s plan built: peers=%d order=%v iter=%d gap=%.4f "+
				"meanLatency=%.1fms directLatency=%.1fms maxUpUtil=%.0f%% maxDownUtil=%.0f%% "+
				"maxRelayUtil=%.0f%% tiers=%v",
			roomID, len(env.Order), env.Order, res.Iterations, res.Gap,
			res.MeanLatency, res.DirectMeanLatency,
			100*res.MaxUpUtil, 100*res.MaxDownUtil, 100*res.MaxRelayUtil,
			env.VideoTiers,
		)
	} else if len(waiting) > 0 {
		log.Printf("[papers/mesh] room=%s plan WAITING on report(s) from: %v", roomID, waiting)
	} else {
		log.Printf("[papers/mesh] room=%s plan not buildable yet (fewer than 2 peers reported)", roomID)
	}
}

func writeMeshPlan(w http.ResponseWriter, env *mesh.PlanEnvelope, waiting []string) {
	w.Header().Set("Content-Type", "application/json")
	if env == nil {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "waiting",
			"waiting": waiting,
		})
		return
	}
	_ = json.NewEncoder(w).Encode(env)
}
