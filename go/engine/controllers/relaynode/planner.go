// Reconciliation and track relay — the server's own analogue of
// CallPlanner.kt/planner.rs's reconcile(), diffing a room's plan against
// what's actually wired and opening/closing connections and forwards to
// match. Relay is pure RTP packet-forwarding (no decode/re-encode): the
// same pattern pion/webrtc's own "broadcast" example uses for an SFU.
package relaynode

import (
	"encoding/json"
	"log"
	"time"

	"github.com/pion/webrtc/v4"

	"tls-rest/go/engine/controllers/mesh"
)

// Reconcile diffs a room's current plan against what's actually wired and
// opens/closes connections and track forwards to match. Called once per
// mesh.Coordinator.Plan() via mesh.RegisterPlanHook (see relaynode.go's
// init), regardless of which module's HTTP handler triggered that Plan().
func (m *Manager) Reconcile(room string, env *mesh.PlanEnvelope) {
	desired := map[string]map[string]bool{} // sourceID -> set of child peerIDs the server must forward it to
	upstream := map[string]string{}         // sourceID -> the peer the server must connect to in order to receive it
	serverPresent := false
	for _, e := range mesh.RelayEdges(env) {
		if e.Parent == mesh.ServerPeerID {
			serverPresent = true
			if desired[e.Source] == nil {
				desired[e.Source] = map[string]bool{}
			}
			desired[e.Source][e.Child] = true
		}
		if e.Child == mesh.ServerPeerID {
			serverPresent = true
			upstream[e.Source] = e.Parent
		}
	}

	rp := m.room(room)
	if !serverPresent {
		m.teardownRoom(room, rp)
		return
	}

	needed := map[string]bool{}
	for _, p := range upstream {
		needed[p] = true
	}
	for _, children := range desired {
		for c := range children {
			needed[c] = true
		}
	}
	for peerID := range needed {
		if _, err := m.getOrCreate(room, peerID); err != nil {
			log.Printf("[relaynode] room=%s connect to %s failed: %v", room, peerID, err)
		}
	}

	type action struct {
		child, source string
		track         *webrtc.TrackRemote
	}

	rp.mu.Lock()
	var stalePeers []string
	for peerID := range rp.peers {
		if !needed[peerID] {
			stalePeers = append(stalePeers, peerID)
		}
	}

	var toSend []action
	for sourceID, children := range desired {
		track := rp.received[sourceID]
		for childID := range children {
			if rp.wiring[sourceID] != nil && rp.wiring[sourceID][childID] {
				continue
			}
			if track == nil {
				continue // not received yet; forwardIfWanted wires it the moment OnTrack sees it
			}
			if rp.wiring[sourceID] == nil {
				rp.wiring[sourceID] = map[string]bool{}
			}
			rp.wiring[sourceID][childID] = true
			toSend = append(toSend, action{child: childID, source: sourceID, track: track})
		}
	}

	var toStop []action
	for sourceID, children := range rp.wiring {
		for childID := range children {
			if desired[sourceID] == nil || !desired[sourceID][childID] {
				delete(children, childID)
				toStop = append(toStop, action{child: childID, source: sourceID})
			}
		}
	}

	rp.lastDesired = desired
	rp.mu.Unlock()

	for _, peerID := range stalePeers {
		m.dropPeer(room, peerID)
	}
	for _, a := range toSend {
		m.sendTrack(room, a.child, a.source, a.track)
	}
	for _, a := range toStop {
		m.stopSource(room, a.child, a.source)
	}
}

// forwardIfWanted is called from OnTrack the moment a source's stream
// actually arrives: Reconcile may already want it forwarded to children but
// couldn't until now, since it only wires a forward once it has a track to
// forward — this fires the wiring immediately rather than waiting for the
// next ~6s report cycle to call Reconcile again.
func (m *Manager) forwardIfWanted(room string, rp *roomPeers, sourceID string, track *webrtc.TrackRemote) {
	rp.mu.Lock()
	rp.received[sourceID] = track
	var toSend []string
	for childID := range rp.lastDesired[sourceID] {
		if rp.wiring[sourceID] != nil && rp.wiring[sourceID][childID] {
			continue
		}
		if rp.wiring[sourceID] == nil {
			rp.wiring[sourceID] = map[string]bool{}
		}
		rp.wiring[sourceID][childID] = true
		toSend = append(toSend, childID)
	}
	rp.mu.Unlock()

	for _, childID := range toSend {
		m.sendTrack(room, childID, sourceID, track)
	}
}

// sendTrack starts forwarding sourceID's track to childID: pure RTP
// packet-forwarding (same packets in, same packets out, no decode/
// re-encode) onto a fresh local track added to childID's own connection,
// plus trackMap so childID's own CallMesh/mesh.ts attributes the relayed
// track to its true origin rather than to the server. A second goroutine
// drains the resulting RTPSender's RTCP feedback (PLI/NACK) so those
// interceptors keep working — same pattern pion's own "broadcast" example
// uses for exactly this case.
func (m *Manager) sendTrack(room, childID, sourceID string, track *webrtc.TrackRemote) {
	ctx, err := m.getOrCreate(room, childID)
	if err != nil {
		log.Printf("[relaynode] sendTrack room=%s child=%s source=%s: connect failed: %v", room, childID, sourceID, err)
		return
	}

	local, err := webrtc.NewTrackLocalStaticRTP(track.Codec().RTPCodecCapability, track.ID(), sourceID+":"+room)
	if err != nil {
		log.Printf("[relaynode] sendTrack room=%s child=%s source=%s: new local track: %v", room, childID, sourceID, err)
		return
	}
	sender, err := ctx.pc.AddTrack(local)
	if err != nil {
		log.Printf("[relaynode] sendTrack room=%s child=%s source=%s: AddTrack: %v", room, childID, sourceID, err)
		return
	}

	ctx.mu.Lock()
	ctx.senders[sourceID] = append(ctx.senders[sourceID], sender)
	ctx.mu.Unlock()

	go drainRTCP(sender)
	go forwardRTP(track, local)
	go sendTrackMap(ctx, local.StreamID(), sourceID)
}

func drainRTCP(sender *webrtc.RTPSender) {
	buf := make([]byte, 1500)
	for {
		if _, _, err := sender.Read(buf); err != nil {
			return
		}
	}
}

func forwardRTP(track *webrtc.TrackRemote, local *webrtc.TrackLocalStaticRTP) {
	for {
		pkt, _, err := track.ReadRTP()
		if err != nil {
			return
		}
		if err := local.WriteRTP(pkt); err != nil {
			return
		}
	}
}

// sendTrackMap waits (bounded) for childID's control channel to open before
// sending — the track add above may still be negotiating when this starts.
func sendTrackMap(ctx *peerCtx, streamID, sourceID string) {
	payload, err := json.Marshal(map[string]string{"t": "trackMap", "streamId": streamID, "source": sourceID})
	if err != nil {
		return
	}
	for i := 0; i < 60; i++ {
		if ctx.control.ReadyState() == webrtc.DataChannelStateOpen {
			_ = ctx.control.SendText(string(payload))
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	log.Printf("[relaynode] %s: control channel never opened, trackMap for source=%s dropped", ctx.peerID, sourceID)
}

// stopSource stops forwarding sourceID's track(s) to childID — the mirror
// of sendTrack, called once Reconcile's desired set no longer includes this
// child for this source.
func (m *Manager) stopSource(room, childID, sourceID string) {
	rp := m.room(room)
	rp.mu.Lock()
	ctx, ok := rp.peers[childID]
	rp.mu.Unlock()
	if !ok {
		return
	}
	ctx.mu.Lock()
	senders := ctx.senders[sourceID]
	delete(ctx.senders, sourceID)
	ctx.mu.Unlock()
	for _, sender := range senders {
		_ = ctx.pc.RemoveTrack(sender)
	}
}

// teardownRoom drops every server<->human connection in room — called once
// a plan no longer includes mesh.ServerPeerID at all (disengaged, or the
// room itself emptied out).
func (m *Manager) teardownRoom(room string, rp *roomPeers) {
	rp.mu.Lock()
	peers := make([]string, 0, len(rp.peers))
	for id := range rp.peers {
		peers = append(peers, id)
	}
	rp.mu.Unlock()
	for _, id := range peers {
		m.dropPeer(room, id)
	}

	m.mu.Lock()
	delete(m.rooms, room)
	m.mu.Unlock()
}
