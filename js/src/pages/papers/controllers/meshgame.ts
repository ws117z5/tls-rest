// MeshGame wires the ported MeshManager (WebRTC mesh, balanced) to the papers
// GameSession. Game messages ride the mesh CONTROL data channel; the balancer
// (planner/probe over the same channel, if enabled) is multiplexed by the "t"
// prefix. Signaling (offer/answer delivery + peer discovery) is injected so this
// stays independent of the exact backend transport.

import { MeshManager } from "../lib/mesh";
import { GameSession, GameMsg, GameTransport, GameView } from "./game";

export interface Signaling {
  clientId: string;
  signal(to: string, data: any): void;                 // deliver signaling to a peer
  onSignal(cb: (from: string, data: any) => void): void; // incoming signaling
  onPeer(cb: (peerId: string) => void): void;          // a peer we should connect to
  onPeerGone?(cb: (peerId: string) => void): void;     // a peer left
}

export class MeshGame {
  readonly mesh: MeshManager;
  readonly session: GameSession;
  private peers = new Set<string>();

  constructor(private sig: Signaling) {
    this.mesh = new MeshManager(sig.clientId, { signal: (to, data) => sig.signal(to, data) });

    const transport: GameTransport = {
      broadcast: (msg) => {
        for (const p of this.peers) this.mesh.sendControl(p, msg as any);
      },
      onMessage: (handler) => {
        this.mesh.onControl = (peer, msg: any) => {
          if (msg && typeof msg.t === "string" && msg.t.startsWith("papers/")) {
            handler(peer, msg as GameMsg);
          }
          // Non-"papers/" control (planner/probe balancing) would be routed to the
          // Prober/Planner here if media balancing is enabled.
        };
      },
      peers: () => [...this.peers],
    };

    this.session = new GameSession(sig.clientId, transport);

    this.mesh.onConnected = (peer) => {
      this.peers.add(peer);
      this.session.peerConnected(peer);
    };
    this.mesh.onClosed = (peer) => {
      this.peers.delete(peer);
      this.session.peerClosed(peer);
    };

    sig.onSignal((from, data) => this.mesh.handleSignal(from, data));
    sig.onPeer((peerId) => this.mesh.connect(peerId));
    sig.onPeerGone?.((peerId) => {
      this.peers.delete(peerId);
      this.session.peerClosed(peerId);
    });
  }

  setSelf(name: string, word: string) {
    this.session.setSelf(name, word);
  }
  onView(cb: (v: GameView) => void) {
    this.session.onChange(cb);
  }
}