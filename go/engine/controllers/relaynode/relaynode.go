// Package relaynode is the Go backend's own WebRTC peer: a direct port of
// CallMesh.kt (papers' web mesh.ts, ported to Android, then to the
// rust-callmesh-rig test rig) specialized to one fixed identity,
// mesh.ServerPeerID ("0") — Perfect Negotiation plus a pre-negotiated
// "control" data channel carrying trackMap attribution for relayed
// tracks, so the server is just another CallMesh-compatible peer to every
// real client, no client-side protocol changes needed. Track relay itself
// (packet forwarding, no decode/re-encode) is in planner.go.
package relaynode

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/pion/interceptor"
	"github.com/pion/webrtc/v4"

	"tls-rest/go/engine/controllers/mesh"
)

const controlLabel = "control"

var sharedAPI = newAPI()

func newAPI() *webrtc.API {
	m := &webrtc.MediaEngine{}
	if err := m.RegisterDefaultCodecs(); err != nil {
		log.Printf("[relaynode] RegisterDefaultCodecs: %v", err)
	}
	i := &interceptor.Registry{}
	if err := webrtc.RegisterDefaultInterceptors(m, i); err != nil {
		log.Printf("[relaynode] RegisterDefaultInterceptors: %v", err)
	}
	return webrtc.NewAPI(webrtc.WithMediaEngine(m), webrtc.WithInterceptorRegistry(i))
}

// peerCtx is one server<->human PeerConnection — the Go analogue of
// CallMesh.kt/mesh.rs's PeerCtx, specialized to the server always being the
// impolite side (mesh.ServerPeerID sorts lexicographically below every real
// user id), so a colliding inbound offer is simply ignored here, never
// rolled back.
type peerCtx struct {
	room    string
	peerID  string
	pc      *webrtc.PeerConnection
	control *webrtc.DataChannel

	mu                sync.Mutex
	makingOffer       bool
	ignoreOffer       bool
	pendingCandidates []webrtc.ICECandidateInit
	streamToSource    map[string]string              // inbound track stream id -> its true source (from a received trackMap)
	senders           map[string][]*webrtc.RTPSender // sourceID -> senders currently forwarding it to this peer
}

// roomPeers is one active call's server-side relay state.
type roomPeers struct {
	mu          sync.Mutex
	peers       map[string]*peerCtx
	received    map[string]*webrtc.TrackRemote // sourceID -> the track currently received for it, from wherever it arrives
	wiring      map[string]map[string]bool     // sourceID -> set of child peerIDs currently being forwarded to
	lastDesired map[string]map[string]bool     // sourceID -> set of child peerIDs Reconcile most recently wanted (see forwardIfWanted)
}

// Manager is the server's relay-node state across every room it has joined
// as mesh.ServerPeerID.
type Manager struct {
	mu    sync.Mutex
	rooms map[string]*roomPeers
}

func NewManager() *Manager { return &Manager{rooms: map[string]*roomPeers{}} }

var defaultManager = NewManager()

// DefaultManager exposes the shared singleton, the same idiom as
// mesh.DefaultCoordinator.
func DefaultManager() *Manager { return defaultManager }

func (m *Manager) room(roomID string) *roomPeers {
	m.mu.Lock()
	defer m.mu.Unlock()
	rp, ok := m.rooms[roomID]
	if !ok {
		rp = &roomPeers{
			peers:       map[string]*peerCtx{},
			received:    map[string]*webrtc.TrackRemote{},
			wiring:      map[string]map[string]bool{},
			lastDesired: map[string]map[string]bool{},
		}
		m.rooms[roomID] = rp
	}
	return rp
}

func (m *Manager) getOrCreate(room, peerID string) (*peerCtx, error) {
	rp := m.room(room)
	rp.mu.Lock()
	if ctx, ok := rp.peers[peerID]; ok {
		rp.mu.Unlock()
		return ctx, nil
	}
	rp.mu.Unlock()

	// No ICEServers: the server has a public IP and no NAT to traverse,
	// unlike the human side (which still gets its usual list from
	// /api/config/ice as today).
	pc, err := sharedAPI.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		return nil, err
	}

	negotiated := true
	var id uint16
	control, err := pc.CreateDataChannel(controlLabel, &webrtc.DataChannelInit{Negotiated: &negotiated, ID: &id})
	if err != nil {
		_ = pc.Close()
		return nil, err
	}

	ctx := &peerCtx{
		room:           room,
		peerID:         peerID,
		pc:             pc,
		control:        control,
		streamToSource: map[string]string{},
		senders:        map[string][]*webrtc.RTPSender{},
	}

	rp.mu.Lock()
	rp.peers[peerID] = ctx
	rp.mu.Unlock()

	m.wireHandlers(room, rp, ctx)

	// A pre-negotiated data channel never fires OnNegotiationNeeded on its
	// own, so one offer/answer round is needed up front — both sides do
	// this at connect time (the human's own CallMesh.kt/mesh.ts does the
	// same), with any resulting glare resolved by HandleSignal below.
	go m.negotiate(ctx)

	return ctx, nil
}

func (m *Manager) wireHandlers(room string, rp *roomPeers, ctx *peerCtx) {
	pc := ctx.pc

	pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			return // end-of-candidates marker
		}
		init := c.ToJSON()
		data, err := json.Marshal(map[string]interface{}{
			"type":          "candidate",
			"candidate":     init.Candidate,
			"sdpMid":        init.SDPMid,
			"sdpMLineIndex": init.SDPMLineIndex,
		})
		if err != nil {
			return
		}
		mesh.QueueSignal(room, ctx.peerID, mesh.ServerPeerID, data)
	})

	pc.OnNegotiationNeeded(func() {
		go m.negotiate(ctx)
	})

	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		switch state {
		case webrtc.PeerConnectionStateFailed, webrtc.PeerConnectionStateClosed, webrtc.PeerConnectionStateDisconnected:
			m.dropPeer(room, ctx.peerID)
		}
	})

	pc.OnTrack(func(track *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		ctx.mu.Lock()
		sourceID, known := ctx.streamToSource[track.StreamID()]
		ctx.mu.Unlock()
		if !known {
			// Arrived before its trackMap control message — dropped rather
			// than buffered, same as CallMesh.kt/mesh.rs: the next Reconcile
			// (or trackMap arriving late and nobody re-sending) would need
			// the publisher to resend, matching the existing client
			// self-healing behavior rather than inventing a new one here.
			return
		}
		m.forwardIfWanted(room, rp, sourceID, track)
	})

	ctx.control.OnMessage(func(msg webrtc.DataChannelMessage) {
		if !msg.IsString {
			return
		}
		var v map[string]interface{}
		if json.Unmarshal(msg.Data, &v) != nil || v["t"] != "trackMap" {
			return
		}
		streamID, _ := v["streamId"].(string)
		source, _ := v["source"].(string)
		if streamID == "" || source == "" {
			return
		}
		ctx.mu.Lock()
		ctx.streamToSource[streamID] = source
		ctx.mu.Unlock()
	})
}

func (m *Manager) negotiate(ctx *peerCtx) {
	ctx.mu.Lock()
	if ctx.makingOffer || ctx.pc.SignalingState() != webrtc.SignalingStateStable {
		// Already offering, or mid-negotiation from the other side's offer —
		// the standard Perfect Negotiation guard. Without it, a second
		// trigger (e.g. OnNegotiationNeeded firing again before the first
		// offer round settles) calls CreateOffer+SetLocalDescription while
		// already in have-local-offer, which pion correctly rejects
		// (confirmed live: "invalid proposed signaling state transition:
		// have-local-offer->SetLocal(offer)->have-local-offer").
		ctx.mu.Unlock()
		return
	}
	ctx.makingOffer = true
	ctx.mu.Unlock()
	defer func() {
		ctx.mu.Lock()
		ctx.makingOffer = false
		ctx.mu.Unlock()
	}()

	offer, err := ctx.pc.CreateOffer(nil)
	if err != nil {
		log.Printf("[relaynode] %s createOffer: %v", ctx.peerID, err)
		return
	}
	if err := ctx.pc.SetLocalDescription(offer); err != nil {
		log.Printf("[relaynode] %s setLocalDescription(offer): %v", ctx.peerID, err)
		return
	}
	data, err := json.Marshal(map[string]string{"type": "offer", "sdp": offer.SDP})
	if err != nil {
		return
	}
	mesh.QueueSignal(ctx.room, ctx.peerID, mesh.ServerPeerID, data)
}

// HandleSignal processes one inbound signaling message from a human
// (fromUserID) addressed to the server — direct port of CallMesh.kt/
// mesh.rs's handle_signal, specialized to the server always being the
// impolite side: a colliding offer from the human is simply ignored,
// trusting the human's own polite-side rollback to retry with the server's
// offer instead.
func (m *Manager) HandleSignal(room, fromUserID string, data json.RawMessage) error {
	ctx, err := m.getOrCreate(room, fromUserID)
	if err != nil {
		return err
	}

	var head struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return err
	}

	switch head.Type {
	case "candidate":
		return m.handleCandidate(ctx, data)
	case "offer", "answer":
		return m.handleDescription(ctx, head.Type, data)
	default:
		return nil
	}
}

func (m *Manager) handleCandidate(ctx *peerCtx, data json.RawMessage) error {
	var c struct {
		Candidate     string  `json:"candidate"`
		SDPMid        *string `json:"sdpMid"`
		SDPMLineIndex *uint16 `json:"sdpMLineIndex"`
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return err
	}
	init := webrtc.ICECandidateInit{Candidate: c.Candidate, SDPMid: c.SDPMid, SDPMLineIndex: c.SDPMLineIndex}

	ctx.mu.Lock()
	if ctx.pc.RemoteDescription() == nil {
		ctx.pendingCandidates = append(ctx.pendingCandidates, init)
		ctx.mu.Unlock()
		return nil
	}
	ignoring := ctx.ignoreOffer
	ctx.mu.Unlock()

	if err := ctx.pc.AddICECandidate(init); err != nil && !ignoring {
		return err
	}
	return nil
}

func (m *Manager) handleDescription(ctx *peerCtx, typeStr string, data json.RawMessage) error {
	var sd struct {
		SDP string `json:"sdp"`
	}
	if err := json.Unmarshal(data, &sd); err != nil {
		return err
	}
	typ := webrtc.SDPTypeAnswer
	if typeStr == "offer" {
		typ = webrtc.SDPTypeOffer
	}
	desc := webrtc.SessionDescription{Type: typ, SDP: sd.SDP}

	ctx.mu.Lock()
	collision := typ == webrtc.SDPTypeOffer && (ctx.makingOffer || ctx.pc.SignalingState() != webrtc.SignalingStateStable)
	ctx.ignoreOffer = collision // always impolite: a collision is ignored, never rolled back
	ctx.mu.Unlock()
	if collision {
		return nil
	}

	if err := ctx.pc.SetRemoteDescription(desc); err != nil {
		return err
	}

	ctx.mu.Lock()
	pending := ctx.pendingCandidates
	ctx.pendingCandidates = nil
	ctx.mu.Unlock()
	for _, c := range pending {
		_ = ctx.pc.AddICECandidate(c)
	}

	if typ != webrtc.SDPTypeOffer {
		return nil
	}
	answer, err := ctx.pc.CreateAnswer(nil)
	if err != nil {
		return err
	}
	if err := ctx.pc.SetLocalDescription(answer); err != nil {
		return err
	}
	out, err := json.Marshal(map[string]string{"type": "answer", "sdp": answer.SDP})
	if err != nil {
		return err
	}
	mesh.QueueSignal(ctx.room, ctx.peerID, mesh.ServerPeerID, out)
	return nil
}

func (m *Manager) dropPeer(room, peerID string) {
	rp := m.room(room)
	rp.mu.Lock()
	ctx, ok := rp.peers[peerID]
	delete(rp.peers, peerID)
	delete(rp.received, peerID) // if peerID was itself a direct source
	for _, children := range rp.wiring {
		delete(children, peerID)
	}
	rp.mu.Unlock()
	if ok {
		_ = ctx.pc.Close()
	}
}

func init() {
	// Runs relaynode's own reconciliation after every mesh.Coordinator.Plan()
	// call, regardless of which module's HTTP handler triggered it — see
	// mesh.RegisterPlanHook.
	mesh.RegisterPlanHook(func(room string, env *mesh.PlanEnvelope) {
		DefaultManager().Reconcile(room, env)
	})
}
