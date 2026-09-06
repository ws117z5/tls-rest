// Types shared in spirit with the Go server (server/types.go) and optimizer.

export interface RoomInfo {
  id: string;
  name: string;
  public: boolean;
  count: number;
  protected: boolean;
}

export interface JoinResp {
  clientId: string;
  token: string;
  room: string;
  peers: Peer[];
}

export interface Peer {
  id: string;
  name: string;
}

export interface SignalEvent {
  from: string;
  data: RTCSessionDescriptionInit | { candidate: RTCIceCandidateInit | null };
}

export interface PeerEvent {
  peer: Peer;
}

export interface StartProbeEvent {
  peers: string[];
}

export interface PeerStat {
  peer: string;
  latencyMs: number;
  upMbps: number;
  downMbps: number;
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
  maxDownUtil: number;
  maxRelayUtil: number;
}

export interface Plan {
  n: number;
  trees: Tree[];
  result: OptResult;
}

export interface PlanEvent {
  order: string[];
  plan: Plan;
  streamBitrateKbps: number;
}

// peer <-> peer control-channel messages
export type Control =
  | { t: "ping"; ts: number }
  | { t: "pong"; ts: number }
  | { t: "thrStart"; bytes: number }
  | { t: "thrDone"; bytes: number; ms: number }
  | { t: "trackMap"; streamId: string; source: string };