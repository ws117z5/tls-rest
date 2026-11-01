import { MeshManager } from "./mesh";
import type { Control, PeerStat } from "./types";

const PINGS = 12;
// 512 KiB, not 2 MiB — probeAll() runs this sequentially per peer (see below),
// so room-wide probe traffic is O(N); keeping each burst small matters once N
// gets into the 10+ range, where the old 2 MiB figure meant tens of MB of pure
// measurement overhead competing with the uplink it's trying to protect.
const THR_BYTES = 512 << 10;
const CHUNK = 16 << 10;
const HIGH_WATER = 1 << 20;

interface RecvState { t0: number; received: number; total: number; }

export interface Report { stats: PeerStat[]; up: number; down: number; }

// Prober measures latency + throughput over each peer's control DataChannel.
export class Prober {
  private pingWaiters = new Map<number, (rtt: number) => void>();
  private thrWaiters = new Map<string, (r: { bytes: number; ms: number }) => void>();
  private recv = new Map<string, RecvState>();
  private downMbps = new Map<string, number>();

  constructor(private mesh: MeshManager) {}

  handle(peerId: string, msg: Control | { t: "__bytes"; bytes: number }) {
    switch (msg.t) {
      case "ping":
        this.mesh.sendControl(peerId, { t: "pong", ts: msg.ts });
        break;
      case "pong": {
        const w = this.pingWaiters.get(msg.ts);
        if (w) { this.pingWaiters.delete(msg.ts); w(performance.now() - msg.ts); }
        break;
      }
      case "thrStart":
        this.recv.set(peerId, { t0: 0, received: 0, total: msg.bytes });
        break;
      case "__bytes": {
        const st = this.recv.get(peerId);
        if (!st) break;
        if (st.received === 0) st.t0 = performance.now();
        st.received += msg.bytes;
        if (st.received >= st.total) {
          const ms = performance.now() - st.t0;
          this.recv.delete(peerId);
          this.downMbps.set(peerId, mbps(st.received, ms));
          this.mesh.sendControl(peerId, { t: "thrDone", bytes: st.received, ms });
        }
        break;
      }
      case "thrDone": {
        const w = this.thrWaiters.get(peerId);
        if (w) { this.thrWaiters.delete(peerId); w({ bytes: msg.bytes, ms: msg.ms }); }
        break;
      }
    }
  }

  private measureLatency(peerId: string): Promise<number> {
    return new Promise((resolve) => {
      const ts = performance.now();
      const timeout = setTimeout(() => { this.pingWaiters.delete(ts); resolve(NaN); }, 3000);
      this.pingWaiters.set(ts, (rtt) => { clearTimeout(timeout); resolve(rtt); });
      this.mesh.sendControl(peerId, { t: "ping", ts });
    });
  }

  private async measureUpload(peerId: string): Promise<number> {
    this.mesh.sendControl(peerId, { t: "thrStart", bytes: THR_BYTES });
    const done = new Promise<{ bytes: number; ms: number }>((resolve) => {
      const timeout = setTimeout(() => resolve({ bytes: 0, ms: 1 }), 15000);
      this.thrWaiters.set(peerId, (r) => { clearTimeout(timeout); resolve(r); });
    });
    const buf = new ArrayBuffer(CHUNK);
    let sent = 0;
    while (sent < THR_BYTES) {
      while (this.mesh.bufferedAmount(peerId) > HIGH_WATER) await sleep(5);
      this.mesh.sendBinary(peerId, buf);
      sent += CHUNK;
    }
    const r = await done;
    return mbps(r.bytes, r.ms);
  }

  async probeAll(peerIds: string[]): Promise<Report> {
    // Latency: tiny ping/pong frames, safe to run concurrently across every
    // peer — unlike a throughput burst this doesn't meaningfully self-congest,
    // so there's no accuracy cost to parallelizing it (was the dominant part of
    // this function's old O(N) *sequential* wall-clock cost, now O(1)).
    const latencies = await Promise.all(
      peerIds.map(async (peer) => {
        let best = Infinity;
        for (let i = 0; i < PINGS; i++) {
          const rtt = await this.measureLatency(peer);
          if (!Number.isNaN(rtt)) best = Math.min(best, rtt);
          await sleep(10);
        }
        return Number.isFinite(best) ? best / 2 : 500;
      }),
    );

    // Throughput stays sequential, one peer at a time — bursts running in
    // parallel would each compete for the same physical uplink and understate
    // every peer's real capacity.
    const stats: PeerStat[] = [];
    for (let i = 0; i < peerIds.length; i++) {
      const peer = peerIds[i];
      const upMbps = await this.measureUpload(peer);
      stats.push({ peer, latencyMs: latencies[i], upMbps, downMbps: this.downMbps.get(peer) ?? 0 });
    }

    const up = Math.max(0, ...stats.map((s) => s.upMbps));
    const down = Math.max(0, ...stats.map((s) => s.downMbps));
    return { stats, up: up || 10, down: down || 50 };
  }
}

function mbps(bytes: number, ms: number): number {
  return ms <= 0 ? 0 : (bytes * 8) / 1e6 / (ms / 1000);
}
function sleep(ms: number) { return new Promise((r) => setTimeout(r, ms)); }