import { MeshManager } from "./mesh";
import type { PlanEvent } from "./types";

export interface RoleSummary {
  publishTo: string[];
  relaying: { source: string; to: string[] }[];
  receiving: { source: string; from: string }[];
  meanLatencyMs: number;
  baselineLatencyMs: number;
  maxUpUtil: number;
}

// Planner turns the per-source distribution trees into publish/relay/render
// actions and reconciles them on every re-plan.
export class Planner {
  private localTracks: MediaStreamTrack[] = [];
  private received = new Map<string, { stream: MediaStream; tracks: MediaStreamTrack[] }>();
  private desired = new Map<string, Set<string>>();
  private sent = new Map<string, Map<string, Set<string>>>();
  private selfId = "";

  onSourceStream: (sourceId: string, stream: MediaStream) => void = () => {};
  onRoles: (r: RoleSummary) => void = () => {};

  constructor(private mesh: MeshManager) {}

  setSelf(id: string) { this.selfId = id; }

  setLocalStream(stream: MediaStream) {
    this.localTracks = stream.getTracks();
    this.onSourceStream(this.selfId, stream);
    this.reconcile(this.selfId);
  }

  handleRemoteTrack(_peerId: string, sourceId: string, stream: MediaStream, track: MediaStreamTrack) {
    let entry = this.received.get(sourceId);
    if (!entry) {
      entry = { stream, tracks: [] };
      this.received.set(sourceId, entry);
      this.onSourceStream(sourceId, stream);
    }
    if (!entry.tracks.includes(track)) entry.tracks.push(track);
    this.reconcile(sourceId);
  }

  applyPlan(msg: PlanEvent) {
    const { order, plan } = msg;
    const selfIdx = order.indexOf(this.selfId);
    if (selfIdx < 0) return;

    const nextDesired = new Map<string, Set<string>>();
    const receivingFrom = new Map<string, string>();
    for (const tree of plan.trees) {
      const sourceId = order[tree.source];
      const kids = tree.children[selfIdx] ?? [];
      if (kids.length > 0) nextDesired.set(sourceId, new Set(kids.map((k) => order[k])));
      const parentIdx = tree.parent[selfIdx];
      if (tree.source !== selfIdx && parentIdx >= 0) receivingFrom.set(sourceId, order[parentIdx]);
    }
    this.desired = nextDesired;

    const sources = new Set<string>([...this.desired.keys(), ...this.sent.keys(), this.selfId]);
    for (const s of sources) this.reconcile(s);

    this.onRoles({
      publishTo: [...(this.desired.get(this.selfId) ?? [])],
      relaying: [...this.desired.entries()]
        .filter(([s]) => s !== this.selfId)
        .map(([source, to]) => ({ source, to: [...to] })),
      receiving: [...receivingFrom.entries()].map(([source, from]) => ({ source, from })),
      meanLatencyMs: plan.result.meanLatency,
      baselineLatencyMs: plan.result.directMeanLatency,
      maxUpUtil: plan.result.maxUpUtil,
    });
  }

  private reconcile(sourceId: string) {
    const desiredChildren = this.desired.get(sourceId) ?? new Set<string>();
    const tracks = sourceId === this.selfId ? this.localTracks : this.received.get(sourceId)?.tracks ?? [];
    let perChild = this.sent.get(sourceId);
    if (!perChild) this.sent.set(sourceId, (perChild = new Map()));
    for (const child of desiredChildren) {
      let sentIds = perChild.get(child);
      if (!sentIds) perChild.set(child, (sentIds = new Set()));
      for (const t of tracks) {
        if (!sentIds.has(t.id)) { this.mesh.sendTrack(child, sourceId, t); sentIds.add(t.id); }
      }
    }
    for (const child of [...perChild.keys()]) {
      if (!desiredChildren.has(child)) { this.mesh.stopSource(child, sourceId); perChild.delete(child); }
    }
  }
}