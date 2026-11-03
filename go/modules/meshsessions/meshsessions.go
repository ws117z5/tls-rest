// Package meshsessions is an admin-only, read-only module listing live
// WebRTC mesh call sessions (go/engine/controllers/mesh's in-memory
// Coordinator) — no table of its own, same DataFetch/NoTable pattern as
// go/modules/msgusers. List mode is the generic grid with filters, one row
// per room (see KeyField); its "view" mode offers a custom "matrix" view
// (js/src/engine/modules/meshsessions/view.matrix.tsx) rendering that
// room's full peer-to-peer adjacency matrix, including the server's own
// retranslator peer (mesh.ServerPeerID) when engaged, plus a timeline
// slider scrubbing through mesh.Coordinator.Timeline's recorded history.
package meshsessions

import (
	"context"
	"net/http"
	"sort"
	"strings"

	"tls-rest/go/app"
	"tls-rest/go/engine/controllers/mesh"
	"tls-rest/go/engine/controllers/request"

	. "tls-rest/go/engine/controllers/field"
	. "tls-rest/go/engine/controllers/module"
)

type MeshSessions struct {
	*ModuleAbstract[interface{}]
}

func (m *MeshSessions) fieldset() []Field {
	return []Field{
		// List: one row per room (see KeyField below) — fetchRows aggregates
		// Coordinator.Snapshot's per-peer rows down to this shape.
		NewField("room", TYPE_STRING, false).WithLabel("Room").AsReadOnly(),
		NewField("peer_count", TYPE_INT, false).WithLabel("Peers").AsReadOnly(),
		NewField("relay_count", TYPE_INT, false).WithLabel("Relaying").AsReadOnly(),
		// Shared with the view-only field below: peer id of the engaged
		// retranslator ("0"), or "" if the server isn't relaying this room.
		NewField("retranslator", TYPE_STRING, false).WithLabel("Retranslator").AsReadOnly(),
		NewField("tier", TYPE_STRING, false).WithLabel("Video Tier(s)").AsReadOnly(),
		NewField("last_seen_ms", TYPE_INT, false).WithLabel("Last Activity (ms ago)").AsReadOnly(),

		// View-only: populated solely by fetchRows' MODE_VIEW branch, read
		// directly by view.matrix.tsx rather than rendered as grid columns.
		NewField("peers", TYPE_STRING, false).WithLabel("Peers").AsReadOnly().InModes(MODE_VIEW),
		NewField("matrix", TYPE_STRING, false).WithLabel("Matrix").AsReadOnly().InModes(MODE_VIEW),
		// Timeline: recent plan history for the scrubber (mesh.Coordinator.Timeline).
		NewField("history", TYPE_STRING, false).WithLabel("History").AsReadOnly().InModes(MODE_VIEW),
	}
}

// filters are display-only: fetchRows reads these params itself, bypassing
// the declarative SQL builder (there's no SQL here at all — see msgusers.go
// for the same pattern).
func (m *MeshSessions) filters() *Filedset {
	return NewFieldset(
		NewFilter("room", TYPE_STRING).WithLabel("Room").Contains(),
		NewFilter("tier", TYPE_STRING).WithLabel("Video Tier").Contains(),
		NewFilter("retranslator", TYPE_CHECKBOX).WithLabel("Retranslator engaged").Equals(),
	)
}

func readOnly(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "meshsessions is read-only", http.StatusMethodNotAllowed)
}

// roomSummary aggregates Coordinator.Snapshot's per-peer rows down to one
// room — see fetchRows.
type roomSummary struct {
	peerCount    int
	relayCount   int
	retranslator bool
	tiers        map[string]bool
	hasLastSeen  bool
	minLastSeen  int64 // most recently active peer's LastSeenMsAgo
}

// fetchRows is the DataFetch hook: MODE_VIEW returns one room's full
// adjacency matrix (keyed by "room", set into the request bag by the
// engine's View handler — see KeyField); any other mode returns one
// aggregated row per room, rows coming from the live Coordinator, not SQL.
func fetchRows(ctx context.Context, r *http.Request, mode int) ([]map[string]interface{}, error) {
	rq := request.From(r)

	if mode == MODE_VIEW {
		room := strings.TrimSpace(rq.String("room"))
		if room == "" {
			return []map[string]interface{}{}, nil
		}
		rm := mesh.DefaultCoordinator().RoomMatrix(room)
		if rm == nil {
			return []map[string]interface{}{}, nil
		}
		return []map[string]interface{}{{
			"room":         rm.Room,
			"peers":        rm.Peers,
			"retranslator": rm.Retranslator,
			"matrix":       rm.Matrix,
			"history":      mesh.DefaultCoordinator().Timeline(room),
		}}, nil
	}

	roomFilter := strings.ToLower(strings.TrimSpace(rq.String("room")))
	tierFilter := strings.ToLower(strings.TrimSpace(rq.String("tier")))
	retranslatorFilter := rq.String("retranslator") == "true"
	filterByRetranslator := rq.String("retranslator") != ""

	byRoom := map[string]*roomSummary{}
	var order []string
	for _, row := range mesh.DefaultCoordinator().Snapshot() {
		s, ok := byRoom[row.Room]
		if !ok {
			s = &roomSummary{tiers: map[string]bool{}}
			byRoom[row.Room] = s
			order = append(order, row.Room)
		}
		s.peerCount++
		if row.Role == "relay" {
			s.relayCount++
		}
		if row.PeerID == mesh.ServerPeerID {
			s.retranslator = true
		}
		if row.Tier != "" {
			s.tiers[row.Tier] = true
		}
		if !s.hasLastSeen || row.LastSeenMsAgo < s.minLastSeen {
			s.minLastSeen = row.LastSeenMsAgo
			s.hasLastSeen = true
		}
	}

	out := make([]map[string]interface{}, 0, len(order))
	for _, room := range order {
		if roomFilter != "" && !strings.Contains(strings.ToLower(room), roomFilter) {
			continue
		}
		s := byRoom[room]
		if filterByRetranslator && s.retranslator != retranslatorFilter {
			continue
		}
		tiers := make([]string, 0, len(s.tiers))
		for t := range s.tiers {
			tiers = append(tiers, t)
		}
		sort.Strings(tiers)
		tierStr := strings.Join(tiers, ", ")
		if tierFilter != "" && !strings.Contains(strings.ToLower(tierStr), tierFilter) {
			continue
		}

		retranslator := ""
		if s.retranslator {
			retranslator = mesh.ServerPeerID
		}
		var lastSeen int64
		if s.hasLastSeen {
			lastSeen = s.minLastSeen
		}

		out = append(out, map[string]interface{}{
			"room":         room,
			"peer_count":   s.peerCount,
			"relay_count":  s.relayCount,
			"retranslator": retranslator,
			"tier":         tierStr,
			"last_seen_ms": lastSeen,
		})
	}
	return out, nil
}

func NewMeshSessions() *MeshSessions {
	m := &MeshSessions{
		ModuleAbstract: &ModuleAbstract[interface{}]{
			ID:                   "meshsessions",
			Name:                 "Mesh Sessions",
			Description:          "Live WebRTC mesh call sessions, one row per room: peer/relay counts, video tier, and retranslator status",
			Icon:                 "statistics",
			Submenu:              "engine",
			ReadOnly:             true,
			KeyField:             "room", // rows have no natural id; "view" a room's full adjacency matrix instead of one peer
			Rights:               make(map[int]int),
			DefaultPermission:    PERMISSION_DENY, // admin-only; no floor grant
			DefaultPermissionSet: true,
			OmitSystemFields:     SystemFieldsExceptID(),
			Overrides: HandlerOverrides{
				Create: readOnly,
				Edit:   readOnly,
				Delete: readOnly,
			},
			DataFetch: fetchRows,
			NoTable:   true,
			CustomViews: map[string]map[string]string{
				"view": {"matrix": "Adjacency Matrix"},
			},
		},
	}
	m.ModuleAbstract.Fields = m.fieldset()
	m.ModuleAbstract.Filters = m.filters()
	return m
}

var Module *MeshSessions

func init() {
	app.RegisterModule(NewMeshSessions(), "meshsessions")
}
