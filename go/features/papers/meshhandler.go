package papers

import (
	"encoding/json"
	"io"
	"net/http"

	"tls-rest/go/lib/mesh"

	"github.com/gorilla/mux"
)

// meshCoordinator holds per-room link reports and turns them into balanced
// relay plans (the optimizer's balancer + resolver).
var meshCoordinator = mesh.NewCoordinator()

// defaultBitrateKbps is the assumed per-stream media bitrate used to size demand
// in the optimizer until the room negotiates its own.
const defaultBitrateKbps = 800

// ReportLink records a peer's measured links and returns the current plan (or a
// "waiting" status while other peers still need to report).
func ReportLink(w http.ResponseWriter, r *http.Request) {
	roomID := mux.Vars(r)["roomId"]

	b, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var req reportRequest
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

	meshCoordinator.Report(roomID, req.Peer, &mesh.Report{
		Up:    req.Up,
		Down:  req.Down,
		Stats: req.Stats,
	})

	env, waiting := meshCoordinator.Plan(roomID, defaultBitrateKbps)
	writeMeshPlan(w, env, waiting)
}

// GetPlan returns the current relay plan for a room, or a waiting status.
func GetPlan(w http.ResponseWriter, r *http.Request) {
	roomID := mux.Vars(r)["roomId"]
	env, waiting := meshCoordinator.Plan(roomID, defaultBitrateKbps)
	writeMeshPlan(w, env, waiting)
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
