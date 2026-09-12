import type { Control } from "./types";

const ICE: RTCConfiguration = {
  iceServers: [{ urls: "stun:stun.l.google.com:19302" }],
  // Add a TURN server here for peers behind symmetric NATs.
};

// The mesh only needs a way to send signaling to a peer; delivery (SSE) is fed
// back in via handleSignal(). This keeps it independent of REST/SSE/WebSocket.
export interface SignalSink {
  signal(to: string, data: unknown): void;
}

interface PeerCtx {
  id: string;
  pc: RTCPeerConnection;
  control: RTCDataChannel;
  polite: boolean;
  makingOffer: boolean;
  ignoreOffer: boolean;
  outStreams: Map<string, MediaStream>;
  senders: Map<string, RTCRtpSender[]>;
  streamToSource: Map<string, string>;
  pendingTracks: Map<string, { stream: MediaStream; track: MediaStreamTrack }[]>;
  // ICE candidates that arrived before the remote description was set (their
  // own signal POST outran the offer's, or the two raced at the server).
  // addIceCandidate() would reject before a remote description exists, so
  // these are held and flushed right after setRemoteDescription succeeds.
  pendingCandidates: RTCIceCandidateInit[];
}

type ControlHandler = (peerId: string, msg: Control) => void;
type TrackHandler = (
  peerId: string,
  sourceId: string,
  stream: MediaStream,
  track: MediaStreamTrack,
) => void;

export class MeshManager {
  private peers = new Map<string, PeerCtx>();
  onControl: ControlHandler = () => {};
  onTrack: TrackHandler = () => {};
  onConnected: (peerId: string) => void = () => {};
  onClosed: (peerId: string) => void = () => {};

  constructor(
    public selfId: string,
    private sink: SignalSink,
  ) {}

  connect(peerId: string): PeerCtx {
    let ctx = this.peers.get(peerId);
    if (ctx) return ctx;

    const pc = new RTCPeerConnection(ICE);
    const polite = this.selfId > peerId;
    const control = pc.createDataChannel("control", { negotiated: true, id: 0 });

    ctx = {
      id: peerId,
      pc,
      control,
      polite,
      makingOffer: false,
      ignoreOffer: false,
      outStreams: new Map(),
      senders: new Map(),
      streamToSource: new Map(),
      pendingTracks: new Map(),
      pendingCandidates: [],
    };
    this.peers.set(peerId, ctx);

    control.binaryType = "arraybuffer";
    control.onopen = () => this.onConnected(peerId);
    control.onmessage = (ev) => this.handleControl(ctx!, ev.data);

    pc.onicecandidate = ({ candidate }) => this.sink.signal(peerId, { candidate });
    pc.onnegotiationneeded = () => this.negotiate(ctx!);
    pc.ontrack = (ev) => this.handleTrack(ctx!, ev);
    pc.onconnectionstatechange = () => {
      if (["failed", "closed", "disconnected"].includes(pc.connectionState)) {
        // Tell the app right away so it can drop the last video frame instead
        // of leaving it frozen on screen.
        this.onClosed(peerId);
      }
      if (pc.connectionState === "failed") {
        console.error(`[mesh] connection to ${peerId} failed (no network path found)`);
      }
      if (pc.connectionState === "failed" || pc.connectionState === "closed") {
        // Terminal (unlike "disconnected", which can self-recover): tear this
        // peer down so a future connect() starts a fresh RTCPeerConnection
        // instead of reusing a dead one forever.
        try { pc.close(); } catch { /* already closed */ }
        this.peers.delete(peerId);
      }
    };

    // A pre-negotiated data channel (both sides agree on it out of band, per
    // the `negotiated: true` above) deliberately does NOT trigger the
    // browser's own onnegotiationneeded — but the underlying connection still
    // needs one offer/answer round to set up ICE/DTLS/SCTP, or the channel
    // never opens. Nothing else would ever kick that off (sendTrack, the only
    // other trigger, only runs once a relay plan exists — and the plan
    // requires probing over this same not-yet-open channel), so trigger it
    // explicitly here. Both peers in a pair do this at connect time; the
    // collision handling in handleSignal resolves the resulting glare.
    this.negotiate(ctx);

    return ctx;
  }

  private async negotiate(ctx: PeerCtx) {
    const pc = ctx.pc;
    try {
      ctx.makingOffer = true;
      await pc.setLocalDescription();
      this.sink.signal(ctx.id, pc.localDescription!);
    } catch (e) {
      console.error("[mesh] negotiation failed", ctx.id, e);
    } finally {
      ctx.makingOffer = false;
    }
  }

  // called by the app when an SSE "signal" event arrives
  async handleSignal(from: string, data: any) {
    const ctx = this.connect(from);
    const pc = ctx.pc;
    try {
      if (data && data.candidate !== undefined && !("type" in data)) {
        if (!pc.remoteDescription) {
          // Arrived before the offer/answer it followed — hold it rather than
          // let addIceCandidate() reject it outright (dropped candidates can
          // be the whole difference between connecting and not).
          if (data.candidate) ctx.pendingCandidates.push(data.candidate);
          return;
        }
        try {
          await pc.addIceCandidate(data.candidate ?? undefined);
        } catch (e) {
          if (!ctx.ignoreOffer) throw e;
        }
        return;
      }
      const desc = data as RTCSessionDescriptionInit;
      const collision = desc.type === "offer" && (ctx.makingOffer || pc.signalingState !== "stable");
      ctx.ignoreOffer = !ctx.polite && collision;
      if (ctx.ignoreOffer) return;
      await pc.setRemoteDescription(desc);
      if (ctx.pendingCandidates.length > 0) {
        const pending = ctx.pendingCandidates;
        ctx.pendingCandidates = [];
        for (const c of pending) {
          try {
            await pc.addIceCandidate(c);
          } catch (e) {
            console.warn(`[mesh] buffered ICE candidate rejected <- ${from}`, e);
          }
        }
      }
      if (desc.type === "offer") {
        await pc.setLocalDescription();
        this.sink.signal(from, pc.localDescription!);
      }
    } catch (e) {
      console.error(`[mesh] handleSignal <- ${from} failed`, e);
      // A genuine (non-ignored — that path returns early above, never throws)
      // SDP application failure leaves this connection stuck: neither side
      // retries on its own, so a pair that hits one of these races on initial
      // connect can go permanently without video. restartIce() forces a fresh
      // negotiationneeded with a brand-new offer covering all current tracks,
      // giving the pair another chance to converge instead of staying silent.
      try { pc.restartIce(); } catch { /* connection already closed */ }
    }
  }

  sendControl(peerId: string, msg: Control) {
    const ctx = this.peers.get(peerId);
    if (ctx && ctx.control.readyState === "open") ctx.control.send(JSON.stringify(msg));
  }
  sendBinary(peerId: string, buf: ArrayBuffer) {
    const ctx = this.peers.get(peerId);
    if (ctx && ctx.control.readyState === "open") ctx.control.send(buf);
  }
  bufferedAmount(peerId: string): number {
    return this.peers.get(peerId)?.control.bufferedAmount ?? 0;
  }

  private handleControl(ctx: PeerCtx, data: any) {
    if (data instanceof ArrayBuffer) {
      this.onControl(ctx.id, { t: "__bytes" as any, bytes: data.byteLength } as any);
      return;
    }
    let msg: Control;
    try {
      msg = JSON.parse(data);
    } catch {
      return;
    }
    if (msg.t === "trackMap") {
      ctx.streamToSource.set(msg.streamId, msg.source);
      const pend = ctx.pendingTracks.get(msg.streamId);
      if (pend) {
        for (const p of pend) this.onTrack(ctx.id, msg.source, p.stream, p.track);
        ctx.pendingTracks.delete(msg.streamId);
      }
      return;
    }
    this.onControl(ctx.id, msg);
  }

  sendTrack(peerId: string, sourceId: string, track: MediaStreamTrack) {
    const ctx = this.connect(peerId);
    let stream = ctx.outStreams.get(sourceId);
    if (!stream) {
      stream = new MediaStream();
      ctx.outStreams.set(sourceId, stream);
    }
    stream.addTrack(track);
    const sender = ctx.pc.addTrack(track, stream);
    const arr = ctx.senders.get(sourceId) ?? [];
    arr.push(sender);
    ctx.senders.set(sourceId, arr);
    const advertise = () =>
      this.sendControl(peerId, { t: "trackMap", streamId: stream!.id, source: sourceId });
    if (ctx.control.readyState === "open") advertise();
    else ctx.control.addEventListener("open", advertise, { once: true });
  }

  stopSource(peerId: string, sourceId: string) {
    const ctx = this.peers.get(peerId);
    if (!ctx) return;
    for (const s of ctx.senders.get(sourceId) ?? []) {
      try {
        ctx.pc.removeTrack(s);
      } catch {
        /* ignore */
      }
    }
    ctx.senders.delete(sourceId);
    ctx.outStreams.delete(sourceId);
  }

  private handleTrack(ctx: PeerCtx, ev: RTCTrackEvent) {
    const stream = ev.streams[0] ?? new MediaStream([ev.track]);
    const source = ctx.streamToSource.get(stream.id);
    if (source) {
      this.onTrack(ctx.id, source, stream, ev.track);
    } else {
      const arr = ctx.pendingTracks.get(stream.id) ?? [];
      arr.push({ stream, track: ev.track });
      ctx.pendingTracks.set(stream.id, arr);
    }
  }

  closeAll() {
    for (const ctx of this.peers.values()) ctx.pc.close();
    this.peers.clear();
  }
}
