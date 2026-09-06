import type { JoinResp, PeerStat, RoomInfo } from "../lib/types";

const api = "/api/rooms";

async function jsonOrThrow(res: Response) {
  if (!res.ok) {
    const msg = await res.text().catch(() => "");
    throw new Error(msg || `HTTP ${res.status}`);
  }
  return res.status === 204 ? null : res.json();
}

// ---- rooms landing-page REST ----

export async function listRooms(): Promise<RoomInfo[]> {
  return jsonOrThrow(await fetch(api));
}

export async function createRoom(
  name: string,
  isPublic: boolean,
  password: string,
): Promise<{ id: string }> {
  return jsonOrThrow(
    await fetch(api, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ name, public: isPublic, password }),
    }),
  );
}

// ---- per-room transport: REST for actions, SSE for server pushes ----

type Handler = (data: any) => void;

export class RoomClient {
  clientId = "";
  token = "";
  private es?: EventSource;
  private handlers = new Map<string, Set<Handler>>();

  constructor(private roomId: string) {}

  on(type: string, h: Handler) {
    let set = this.handlers.get(type);
    if (!set) this.handlers.set(type, (set = new Set()));
    set.add(h);
    return () => set!.delete(h);
  }
  private emit(type: string, data: any) {
    this.handlers.get(type)?.forEach((h) => h(data));
  }

  async join(name: string, password: string): Promise<JoinResp> {
    const resp: JoinResp = await jsonOrThrow(
      await fetch(`${api}/${this.roomId}/join`, {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ name, password }),
      }),
    );
    this.clientId = resp.clientId;
    this.token = resp.token;
    return resp;
  }

  // open the SSE stream that delivers signal/peerJoined/peerLeft/plan/startProbe
  connect() {
    const url = `${api}/${this.roomId}/events?token=${encodeURIComponent(this.token)}`;
    this.es = new EventSource(url);
    this.es.onmessage = (ev) => {
      try {
        const env = JSON.parse(ev.data);
        this.emit(env.type, env.data);
      } catch {
        /* keep-alive comment or malformed line */
      }
    };
    this.es.onerror = () => this.emit("error", null);
  }

  signal(to: string, data: unknown) {
    // fire-and-forget POST; ordering is preserved by the single SSE consumer
    void fetch(`${api}/${this.roomId}/signal`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ token: this.token, to, data }),
    });
  }

  report(stats: PeerStat[], up: number, down: number) {
    void fetch(`${api}/${this.roomId}/report`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ token: this.token, stats, up, down }),
    });
  }

  leave() {
    this.es?.close();
    // best-effort; keepalive so it still sends during unload
    void fetch(`${api}/${this.roomId}/leave`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ token: this.token }),
      keepalive: true,
    });
  }
}