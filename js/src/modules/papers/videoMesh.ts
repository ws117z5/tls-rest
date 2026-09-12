import axios from "axios";
import { MeshManager } from "../../pages/papers/lib/mesh";
import { Planner } from "../../pages/papers/lib/planner";
import { Prober } from "../../pages/papers/lib/probe";
import type { Control, PlanEvent } from "../../pages/papers/lib/types";

// RoomMesh wires the reused WebRTC mesh engine (js/src/pages/papers/lib:
// MeshManager for perfect-negotiation peer connections, Planner for turning a
// relay plan into actual publish/relay track routing, Prober for the
// latency/throughput measurements that plan is built from) into the live
// papers game room.
//
// Signaling rides a tiny queue-based relay — POST/GET /papers/{hash}/game/signal
// (signal.go) — fed by the room's existing SSE "changed" ping (the same one
// join/turn/state changes already use). Bandwidth/latency reporting and relay
// planning use the module's existing /papers/{hash}/report and
// /papers/{hash}/plan endpoints (the Go mesh optimizer, already wired to
// meshhandler.go — unrelated to and untouched by the room's turn/word game).
// The control data channel doubles as the room's live chat transport (chat.ts).
export class RoomMesh {
  readonly mesh: MeshManager;
  private planner: Planner;
  private prober: Prober;
  private roomHash: string;
  private knownPeers = new Set<string>();
  private planTimer: number | null = null;
  private closed = false;

  onStream: (sourceId: string, stream: MediaStream) => void = () => {};
  // Fired when a peer's connection breaks (logged out, closed the tab, network
  // drop, ...): the app should drop that peer's video immediately rather than
  // leave the last frame frozen on screen.
  onPeerClosed: (peerId: string) => void = () => {};
  // A chat message arrived from a peer (see sendChat below).
  onChat: (fromId: string, text: string, ts: number) => void = () => {};

  constructor(selfId: string, roomHash: string) {
    this.roomHash = roomHash;
    this.mesh = new MeshManager(selfId, {
      signal: (to, data) => {
        axios.post(`/papers/${roomHash}/game/signal`, { to, data }).catch((e) =>
          console.error(`[room-mesh] POST /game/signal -> ${to} failed`, e?.message)
        );
      },
    });
    this.planner = new Planner(this.mesh);
    this.planner.setSelf(selfId);
    this.prober = new Prober(this.mesh);

    this.mesh.onControl = (peer, msg) => {
      if (msg.t === "chat") {
        this.onChat(peer, msg.text, msg.ts);
        return;
      }
      this.prober.handle(peer, msg as any);
    };
    this.mesh.onTrack = (peer, source, stream, track) =>
      this.planner.handleRemoteTrack(peer, source, stream, track);
    this.mesh.onConnected = () => this.reportAndPlan();
    this.mesh.onClosed = (peer) => {
      // Let a future syncPeers() reconnect instead of treating this peer as
      // permanently done — we have no explicit "left the room" signal, so a
      // dropped connection and a temporary network blip look the same here.
      this.knownPeers.delete(peer);
      this.onPeerClosed(peer);
    };
    this.planner.onSourceStream = (id, stream) => this.onStream(id, stream);

    // Pick up plans other peers' own reports produced, even when we haven't
    // reported ourselves recently (bandwidth/latency drift over time).
    this.planTimer = window.setInterval(() => this.fetchPlan(), 6000);
  }

  // Publishes the local camera as this room's "self" source. The planner only
  // actually sends it to peers once a plan says to (see reportAndPlan/applyPlan).
  setLocalStream(stream: MediaStream) {
    this.planner.setLocalStream(stream);
  }

  // Call whenever the room's player roster changes (from /game/state polling).
  // Connecting kicks off perfect-negotiation automatically (MeshManager's
  // onnegotiationneeded) — no explicit offer/answer call needed here.
  syncPeers(peerIds: string[]) {
    for (const id of peerIds) {
      if (this.knownPeers.has(id)) continue;
      this.knownPeers.add(id);
      this.mesh.connect(id);
    }
  }

  // Feed one drained signaling message to the mesh.
  handleSignal(from: string, data: any) {
    this.mesh.handleSignal(from, data);
  }

  // Broadcasts a chat message to every connected peer over the mesh's control
  // channel — no separate chat backend/signaling needed.
  sendChat(text: string) {
    const msg: Control = { t: "chat", text, ts: Date.now() };
    for (const peer of this.knownPeers) this.mesh.sendControl(peer, msg);
  }

  private async reportAndPlan() {
    const peers = [...this.knownPeers];
    if (peers.length === 0) return;
    try {
      const report = await this.prober.probeAll(peers);
      const res = await axios.post(`/papers/${this.roomHash}/report`, {
        peer: this.mesh.selfId,
        up: report.up,
        down: report.down,
        stats: report.stats.map((s) => ({
          peer: s.peer,
          latencyMs: s.latencyMs,
          upMbps: s.upMbps,
        })),
      });
      this.applyPlanResponse(res.data);
    } catch (e: any) {
      console.error("[room-mesh] reportAndPlan failed", e?.message, e?.response?.status);
    }
  }

  private async fetchPlan() {
    if (this.closed || this.knownPeers.size === 0) return;
    try {
      const res = await axios.get(`/papers/${this.roomHash}/plan`);
      this.applyPlanResponse(res.data);
    } catch (e: any) {
      console.error("[room-mesh] fetchPlan failed", e?.message, e?.response?.status);
    }
  }

  private applyPlanResponse(data: any) {
    if (!data || data.status === "waiting" || !data.plan) return;
    this.planner.applyPlan(data as PlanEvent);
  }

  close() {
    if (this.closed) return;
    this.closed = true;
    if (this.planTimer != null) window.clearInterval(this.planTimer);
    this.mesh.closeAll();
  }
}
