package mesh

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

var coordinator = NewCoordinator()

// ReportRequest is the body a peer POSTs to /{module}/{roomId}/report.
type ReportRequest struct {
	Peer  string     `json:"peer"`
	Up    float64    `json:"up"`
	Down  float64    `json:"down"`
	Stats []LinkStat `json:"stats"`
}

// ReportLink records a peer's measured links and returns the current plan.
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

	log.Printf("[mesh] room=%s report from %s: up=%.1f down=%.1f stats=%d link(s)",
		roomID, req.Peer, req.Up, req.Down, len(req.Stats))

	coordinator.Report(roomID, req.Peer, &Report{
		Up:    req.Up,
		Down:  req.Down,
		Stats: req.Stats,
	})

	env, waiting := coordinator.Plan(roomID)
	logPlanResult(roomID, env, waiting)
	writeMeshPlan(w, env, waiting)
}

// GetPlan returns the current relay plan for a room, or a waiting status.
func GetPlan(w http.ResponseWriter, r *http.Request) {
	roomID := mux.Vars(r)["roomId"]
	env, waiting := coordinator.Plan(roomID)
	writeMeshPlan(w, env, waiting)
}

// logPlanResult logs a (re)built plan; called from ReportLink only, not the GetPlan poll.
func logPlanResult(roomID string, env *PlanEnvelope, waiting []string) {
	if env != nil {
		res := env.Plan.Result
		log.Printf(
			"[mesh] room=%s plan built: peers=%d order=%v iter=%d gap=%.4f "+
				"meanLatency=%.1fms directLatency=%.1fms maxUpUtil=%.0f%% maxDownUtil=%.0f%% "+
				"maxRelayUtil=%.0f%% tiers=%v",
			roomID, len(env.Order), env.Order, res.Iterations, res.Gap,
			res.MeanLatency, res.DirectMeanLatency,
			100*res.MaxUpUtil, 100*res.MaxDownUtil, 100*res.MaxRelayUtil,
			env.VideoTiers,
		)
	} else if len(waiting) > 0 {
		log.Printf("[mesh] room=%s plan WAITING on report(s) from: %v", roomID, waiting)
	} else {
		log.Printf("[mesh] room=%s plan not buildable yet (fewer than 2 peers reported)", roomID)
	}
}

func writeMeshPlan(w http.ResponseWriter, env *PlanEnvelope, waiting []string) {
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
