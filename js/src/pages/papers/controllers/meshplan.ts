// Papers mesh client.
//
// Bridges the papers WebRTC room to the Go optimizer (balancer + resolver):
// peers POST their measured links, GET the balanced plan, and resolve that plan
// into concrete publish / relay / receive actions for themselves.
//
// The media wiring (attaching forwarded tracks to the room's peer connections)
// lives in the room components; rolesFor() below is the pure decision layer.

import axios from "axios";
import Config from "@engine/Config";

export interface LinkStat {
    peer: string;
    latencyMs: number;
    upMbps: number;
}

export interface MeshReport {
    peer: string;
    up: number; // uplink Mbit/s
    down: number; // downlink Mbit/s
    stats: LinkStat[];
}

export interface Tree {
    source: number;
    parent: number[];
    children: number[][];
    hopCount: number[];
}

export interface OptResult {
    meanLatency: number;
    directMeanLatency: number;
    maxUpUtil: number;
}

export interface Plan {
    n: number;
    trees: Tree[];
    result: OptResult;
}

export interface PlanEnvelope {
    order: string[];
    plan: Plan;
    streamBitrateKbps: number;
}

export interface PlanResponse {
    status?: "waiting";
    waiting?: string[];
    // when ready, PlanEnvelope fields are present:
    order?: string[];
    plan?: Plan;
    streamBitrateKbps?: number;
}

/** POST this peer's measured links; the response is the current plan or a wait. */
export async function reportLinks(roomId: string, report: MeshReport): Promise<PlanResponse> {
    const res = await axios.post(`${Config.serverURL}papers/${roomId}/report`, report);
    return res.data as PlanResponse;
}

/** GET the current relay plan for a room (or a waiting status). */
export async function fetchPlan(roomId: string): Promise<PlanResponse> {
    const res = await axios.get(`${Config.serverURL}papers/${roomId}/plan`);
    return res.data as PlanResponse;
}

export interface Roles {
    // streams this peer originates and sends directly to these peers
    publishTo: string[];
    // sources this peer forwards on behalf of others
    relaying: { source: string; to: string[] }[];
    // sources this peer pulls, and from which upstream peer
    receiving: { source: string; from: string }[];
}

// rolesFor resolves the optimizer's per-source trees into what `selfPeerId` must
// do: publish its own stream to its direct children, relay other sources to the
// children it parents, and receive each other source from its parent. This is
// the client-side half of the resolver — pure and unit-testable.
export function rolesFor(order: string[], plan: Plan, selfPeerId: string): Roles {
    const selfIdx = order.indexOf(selfPeerId);
    const roles: Roles = { publishTo: [], relaying: [], receiving: [] };
    if (selfIdx < 0) return roles;

    for (const tree of plan.trees) {
        const sourceId = order[tree.source];
        const kids = (tree.children[selfIdx] || []).map((k) => order[k]);
        const parentIdx = tree.parent[selfIdx];

        if (tree.source === selfIdx) {
            // Our own stream: children are the peers we publish to directly.
            if (kids.length > 0) roles.publishTo = kids;
        } else {
            if (kids.length > 0) roles.relaying.push({ source: sourceId, to: kids });
            if (parentIdx >= 0) roles.receiving.push({ source: sourceId, from: order[parentIdx] });
        }
    }
    return roles;
}