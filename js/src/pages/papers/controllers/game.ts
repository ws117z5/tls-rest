// Papers game logic — the "word on the forehead" mechanic, decoupled from the
// transport so it can run over the WebRTC mesh (MeshManager control channel) and
// be unit-tested in isolation.
//
// Rules:
//   - every player submits a word,
//   - each player is ASSIGNED a word submitted by a DIFFERENT player,
//   - a player must NOT see the word assigned to them (it's on their forehead),
//   - a player sees the words assigned to everyone else.

export interface Player {
  id: string;
  name: string;
  word: string;   // the word THIS player submitted
  ready: boolean; // has submitted a non-empty word
}

export interface Assignment {
  forClient: string;   // who "wears" this word (can't see it)
  word: string;        // the assigned word
  submittedBy: string; // who submitted it
}

// assignWords produces a derangement: each ready player receives a word submitted
// by another player, and never their own. It shuffles then rotates by one, which
// guarantees no self-assignment for n >= 2.
export function assignWords(players: Player[], rng: () => number = Math.random): Assignment[] {
  const ready = players.filter((p) => p.ready && p.word.trim() !== "");
  const n = ready.length;
  if (n < 2) return [];
  const s = [...ready];
  for (let i = n - 1; i > 0; i--) {
    const j = Math.floor(rng() * (i + 1));
    [s[i], s[j]] = [s[j], s[i]];
  }
  return s.map((p, i) => {
    const owner = s[(i + 1) % n]; // rotation -> never yourself
    return { forClient: p.id, word: owner.word, submittedBy: owner.id };
  });
}

// visibleTo returns the assignments a given client is allowed to see: everyone
// else's forehead word, never their own.
export function visibleTo(self: string, assignments: Assignment[]): Assignment[] {
  return assignments.filter((a) => a.forClient !== self);
}

// electHost picks a deterministic host among the connected ids (lowest id wins),
// so exactly one peer computes and broadcasts the assignment without a server.
export function electHost(ids: string[]): string {
  return [...ids].sort()[0] ?? "";
}

// ---- wire protocol over the mesh control channel ------------------------

export type GameMsg =
  | { t: "papers/hello"; id: string; name: string; word: string; ready: boolean }
  | { t: "papers/assign"; assignments: Assignment[]; round: number };

// Transport is whatever can broadcast a GameMsg to all peers and deliver
// incoming ones — implemented by the mesh in the component (see integration).
export interface GameTransport {
  broadcast(msg: GameMsg): void;
  onMessage(handler: (from: string, msg: GameMsg) => void): void;
  peers(): string[]; // connected peer ids (excluding self is fine; self added below)
}

export interface GameView {
  self: Player;
  players: Map<string, Player>;
  round: number;
  visible: Assignment[]; // words this client should see (others' foreheads)
  myWordHidden: boolean; // true once a word has been assigned to me
}

// GameSession glues the rules to a transport. Each client gossips its hello; the
// elected host, once every known peer is ready, computes the assignment and
// broadcasts it. Everyone renders visibleTo(self).
export class GameSession {
  private players = new Map<string, Player>();
  private assignments: Assignment[] = [];
  private round = 0;
  private onChangeCb: (v: GameView) => void = () => {};

  constructor(
    private selfId: string,
    private transport: GameTransport,
  ) {
    this.players.set(selfId, { id: selfId, name: "", word: "", ready: false });
    transport.onMessage((from, msg) => this.handle(from, msg));
  }

  onChange(cb: (v: GameView) => void) {
    this.onChangeCb = cb;
    this.emit();
  }

  // Local player sets their name / word; re-broadcast hello and try to (re)assign.
  setSelf(name: string, word: string) {
    const me = this.players.get(this.selfId)!;
    me.name = name;
    me.word = word;
    me.ready = word.trim() !== "";
    this.broadcastHello();
    this.maybeAssign();
    this.emit();
  }

  // Called by the component when a peer connects/disconnects so we (re)gossip.
  peerConnected(_peerId: string) {
    this.broadcastHello();
    this.maybeAssign();
  }
  peerClosed(peerId: string) {
    this.players.delete(peerId);
    this.emit();
  }

  private broadcastHello() {
    const me = this.players.get(this.selfId)!;
    this.transport.broadcast({
      t: "papers/hello",
      id: me.id,
      name: me.name,
      word: me.word,
      ready: me.ready,
    });
  }

  private handle(from: string, msg: GameMsg) {
    if (msg.t === "papers/hello") {
      this.players.set(msg.id, { id: msg.id, name: msg.name, word: msg.word, ready: msg.ready });
      // A newcomer needs our hello too.
      this.broadcastHello();
      this.maybeAssign();
      this.emit();
    } else if (msg.t === "papers/assign") {
      if (msg.round >= this.round) {
        this.assignments = msg.assignments;
        this.round = msg.round;
        this.emit();
      }
    }
  }

  // Host-only: when every known player is ready, derange and broadcast.
  private maybeAssign() {
    const ids = [...this.players.keys()];
    if (electHost(ids) !== this.selfId) return; // only the host assigns
    const all = [...this.players.values()];
    if (all.length < 2 || !all.every((p) => p.ready)) return;
    // Don't reassign an identical, already-published round.
    const next = assignWords(all);
    if (next.length === 0) return;
    this.round += 1;
    this.assignments = next;
    this.transport.broadcast({ t: "papers/assign", assignments: next, round: this.round });
    this.emit();
  }

  private emit() {
    const self = this.players.get(this.selfId)!;
    const mine = this.assignments.find((a) => a.forClient === this.selfId);
    this.onChangeCb({
      self,
      players: new Map(this.players),
      round: this.round,
      visible: visibleTo(this.selfId, this.assignments),
      myWordHidden: !!mine,
    });
  }
}