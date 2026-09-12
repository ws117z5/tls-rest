import React, { useCallback, useEffect, useRef, useState } from "react";
import axios from "axios";
import { ModuleViewProps } from "@engine/controllers/registry";
import { RoomMesh } from "./videoMesh";
import "./papers.css";

interface PlayerView { key: string; name: string; word?: string; ready: boolean; active: boolean; finished: boolean; }
interface ScoreEntry { key: string; name: string; }
interface GameState {
  selfName: string; selfWord: string; selfFinished: boolean; isCreator: boolean;
  players: PlayerView[]; scores: ScoreEntry[]; gameOver: boolean;
  activeKey: string; started: boolean;
  wrong: number; wrongLimit: number; timerSecs: number; remaining: number;
  deadline: number; selfKey: string;
}

// A square video tile. `stream` is attached whenever it changes — the same
// mechanism for the local camera and every remote participant's mesh-received
// stream (see RoomMesh.onStream in videoMesh.ts). The assigned word shows
// ABOVE the tile for everyone except the player wearing it.
const Tile: React.FC<{ pv: PlayerView; self: boolean; active: boolean; stream?: MediaStream }> = ({
  pv,
  self,
  active,
  stream,
}) => {
  const ref = useRef<HTMLVideoElement>(null);
  useEffect(() => {
    if (ref.current) ref.current.srcObject = stream ?? null;
  }, [stream]);
  return (
    <div className={`papers-tile ${active ? "active" : ""} ${self ? "you" : ""}`}>
      {!self && pv.word ? <div className="papers-word">{pv.word}</div> : null}
      <video id={pv.key} ref={ref} className="papers-video" autoPlay playsInline muted={self} />
      <span className="papers-name">
        {pv.name || pv.key.slice(0, 6)}{active ? " ⏳" : ""}{pv.finished ? " ✅" : ""}
      </span>
    </div>
  );
};

const PapersView: React.FC<ModuleViewProps> = ({ record, module, navigate }) => {
  const room = record || {};
  // Rooms are addressed by their stored hash (KeyField), never uuid or id.
  const roomId = room.hash;
  const base = `/papers/${roomId}/game`;

  const [state, setState] = useState<GameState | null>(null);
  const [name, setName] = useState("");
  const [word, setWord] = useState("");
  const [countdown, setCountdown] = useState(0);

  // Local camera + remote participants' streams, keyed by player key ("source
  // id" in mesh terms). RoomMesh owns the actual peer connections; this
  // component just renders whatever it hands back.
  const [localStream, setLocalStream] = useState<MediaStream | null>(null);
  const [remoteStreams, setRemoteStreams] = useState<Record<string, MediaStream>>({});
  const meshRef = useRef<RoomMesh | null>(null);

  // Live chat, carried over the same WebRTC mesh control channel used for
  // signaling/probing (see RoomMesh.sendChat) — no separate chat backend.
  const [chatMessages, setChatMessages] = useState<{ fromId: string; text: string; ts: number }[]>([]);
  const [chatInput, setChatInput] = useState("");

  // Password gate. Re-created on every entry (component remounts when you leave
  // and re-open the room), so a protected room always re-prompts.
  const protectedRoom = !!record?.has_password;
  const [joined, setJoined] = useState<boolean>(!protectedRoom);
  const [password, setPassword] = useState("");
  const [pwError, setPwError] = useState("");

  const attemptJoin = async () => {
    setPwError("");
    try {
      await axios.post(`/papers/${roomId}`, { uuid: "self", name: name.trim() || "player", password });
      setJoined(true);
    } catch (e: any) {
      if (e?.response?.status === 403) {
        navigate(`/${module}`); // wrong password -> back to the list
      } else {
        setPwError("Could not join the room.");
      }
    }
  };

  // Poll game state (turn / timer / assignments).
  const refresh = useCallback(async () => {
    try {
      const res = await axios.get(`${base}/state`);
      setState(res.data);
    } catch { /* */ }
  }, [base]);

  // Drain any WebRTC signaling messages (offer/answer/ICE) queued for us —
  // see signal.go. Delivery piggybacks on the same "changed" SSE ping the game
  // state already uses, so this runs right alongside refresh().
  const drainSignals = useCallback(async () => {
    if (!meshRef.current) return;
    try {
      const res = await axios.get(`${base}/signal`);
      const messages = res.data?.messages || [];
      // Awaited in order, not fired concurrently: an offer must finish
      // setRemoteDescription() before any ICE candidate that followed it is
      // applied, or addIceCandidate() races ahead and silently corrupts the
      // handshake (candidates for a not-yet-set remote description).
      for (const m of messages) {
        await meshRef.current.handleSignal(m.from, m.data);
      }
    } catch (e: any) {
      console.error("[papers] drainSignals failed", e?.message);
    }
  }, [base]);

  // Subscribe to live updates (SSE): the server pushes a "changed" event on any
  // state change (join / start / wrong / turn expiry / signal) and we pull our
  // own state (and drain any queued signaling).
  useEffect(() => {
    let done = false;
    refresh().then(() => {
      if (done) return;
      setState((s) => {
        if (s) { if (!name && s.selfName) setName(s.selfName); if (!word && s.selfWord) setWord(s.selfWord); }
        return s;
      });
    });
    const es = new EventSource(`${base}/events`);
    es.onmessage = () => { refresh(); drainSignals(); };
    // EventSource auto-reconnects on error; a slow fallback poll covers gaps.
    const fallback = setInterval(() => { refresh(); drainSignals(); }, 10000);
    return () => { done = true; es.close(); clearInterval(fallback); };
  }, [refresh, drainSignals, base]);

  // Local 1s countdown, re-synced from server `remaining` on each poll.
  useEffect(() => {
    if (!state?.started) { setCountdown(0); return; }
    setCountdown(state.remaining);
    const id = setInterval(() => setCountdown((c) => Math.max(0, c - 1)), 1000);
    return () => clearInterval(id);
  }, [state?.started, state?.remaining, state?.activeKey]);

  // Acquire the local camera once joined. If the tile unmounts (Back button /
  // navigating away) before the permission prompt resolves, the stream would
  // otherwise arrive after cleanup already ran and never get stopped, leaving
  // the camera on — the `cancelled` flag stops it immediately in that case.
  useEffect(() => {
    if (!joined) return;
    let stream: MediaStream | null = null;
    let cancelled = false;
    navigator.mediaDevices.getUserMedia({ video: true, audio: false })
      .then((s) => {
        if (cancelled) { s.getTracks().forEach((t) => t.stop()); return; }
        stream = s;
        setLocalStream(s);
      })
      .catch((e) => console.error("[papers] getUserMedia failed", e?.name, e?.message));
    return () => {
      cancelled = true;
      stream?.getTracks().forEach((t) => t.stop());
      setLocalStream(null);
    };
  }, [joined]);

  // Create the room's WebRTC mesh once our identity is known, and tear it down
  // (closing every peer connection) when we leave.
  useEffect(() => {
    if (!joined || !state?.selfKey || !roomId) return;
    const selfKey = state.selfKey;
    const rm = new RoomMesh(selfKey, roomId);
    rm.onStream = (id, stream) => {
      if (id === selfKey) return; // self renders localStream directly, not via the mesh
      setRemoteStreams((prev) => ({ ...prev, [id]: stream }));
    };
    rm.onPeerClosed = (id) => {
      // Connection broke (logged out, closed the tab, network drop, ...) —
      // drop the stream so the tile goes black instead of freezing on the
      // last frame it received.
      setRemoteStreams((prev) => {
        if (!(id in prev)) return prev;
        const next = { ...prev };
        delete next[id];
        return next;
      });
    };
    rm.onChat = (fromId, text, ts) => {
      setChatMessages((prev) => [...prev, { fromId, text, ts }].slice(-200));
    };
    meshRef.current = rm;
    return () => {
      rm.close();
      meshRef.current = null;
      setRemoteStreams({});
    };
  }, [joined, state?.selfKey, roomId]);

  // Hand the local camera to the mesh once both it and the mesh exist —
  // whichever becomes ready second picks up the other via the ref.
  useEffect(() => {
    if (localStream && meshRef.current) meshRef.current.setLocalStream(localStream);
  }, [localStream, state?.selfKey]);

  // Keep the mesh's peer connections in sync with the room roster. Declared
  // after the mesh-creation effect above so meshRef.current is already set
  // within the same render commit.
  useEffect(() => {
    if (!meshRef.current || !state?.players) return;
    const peers = state.players.map((p) => p.key).filter((k) => k !== state.selfKey);
    meshRef.current.syncPeers(peers);
  }, [state?.players, state?.selfKey]);

  const join = async () => {
    await axios.post(`${base}/join`, { name: name.trim(), word: word.trim() });
    refresh();
  };
  const turn = async (action: string) => {
    await axios.post(`${base}/turn`, { action });
    if (action === "end") { navigate(`/${module}`); return; }
    refresh();
  };
  // Self-reported: the player says the word out loud over video and everyone
  // else confirms verbally, then clicks this. Removes them from the active-
  // turn rotation and records their finish position on the scoreboard.
  const guessWord = async () => {
    await axios.post(`${base}/guess`, {});
    refresh();
  };

  const players = state?.players ?? [];
  const selfKey = state?.selfKey;
  const mmss = (s: number) => `${String(Math.floor(s / 60)).padStart(2, "0")}:${String(s % 60).padStart(2, "0")}`;
  const nameFor = (key: string) => players.find((p) => p.key === key)?.name || key.slice(0, 6);

  const sendChat = () => {
    const text = chatInput.trim();
    if (!text || !meshRef.current || !selfKey) return;
    meshRef.current.sendChat(text);
    // sendChat only reaches other peers over their control channels — add our
    // own message to the local view too.
    setChatMessages((prev) => [...prev, { fromId: selfKey, text, ts: Date.now() }].slice(-200));
    setChatInput("");
  };

  // Popup password gate for protected rooms.
  if (!joined) {
    return (
      <div className="papers-room">
        <div className="papers-room-header">
          <h4 className="mb-0">{room.name || "Room"}</h4>
        </div>
        <div className="papers-modal-backdrop">
          <div className="papers-modal card shadow">
            <div className="card-body">
              <h5 className="card-title mb-3">Password required</h5>
              <p className="text-muted small mb-2">This game is password-protected.</p>
              <label className="form-label mb-1">Your name</label>
              <input className="form-control mb-2" value={name} onChange={(e) => setName(e.target.value)} />
              <label className="form-label mb-1">Password</label>
              <input
                className="form-control mb-2"
                type="password"
                autoFocus
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                onKeyDown={(e) => { if (e.key === "Enter") attemptJoin(); }}
              />
              {pwError && <div className="text-danger small mb-2">{pwError}</div>}
              <div className="d-flex justify-content-end gap-2 mt-2">
                <button className="btn btn-secondary" onClick={() => navigate(`/${module}`)}>Cancel</button>
                <button className="btn btn-primary" onClick={attemptJoin} disabled={!password}>Enter</button>
              </div>
            </div>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="papers-room">
      <div className="papers-room-header">
        <h4 className="mb-0">{room.name || "Room"}</h4>
        <div className="d-flex align-items-center gap-3">
          {/* No in-room Leave button: the page chrome's Back button (above) is
              the one way out, and it unmounts this view, which stops the
              camera and closes every peer connection. */}
          {state?.started && <span className="papers-timer">{mmss(countdown)}</span>}
        </div>
      </div>

      {/* Name + word entry (prefilled). Word is assigned to another player. */}
      <div className="papers-controls d-flex flex-wrap gap-2 align-items-end p-3">
        <div>
          <label className="form-label mb-1">Your name</label>
          <input className="form-control" style={{ maxWidth: 180 }} value={name} onChange={(e) => setName(e.target.value)} />
        </div>
        <div>
          <label className="form-label mb-1">Your word</label>
          <input className="form-control" style={{ maxWidth: 200 }} value={word} onChange={(e) => setWord(e.target.value)}
                 placeholder="for someone else" />
        </div>
        <button className="btn btn-primary" onClick={join} disabled={!name.trim() || !word.trim()}>Set</button>

        {/* Self-reported: say the word out loud, everyone else confirms, then
            click this. Available any time, not just on your turn — it just
            takes you out of the active-turn queue and records your finish. */}
        <button
          className="btn btn-outline-success"
          onClick={guessWord}
          disabled={!state || state.selfFinished}
        >
          {state?.selfFinished ? "You guessed it! ✅" : "I guessed my word!"}
        </button>

        {/* Creator-only game controls. */}
        {state?.isCreator && (
          <div className="ms-auto d-flex gap-2">
            <button className="btn btn-success" onClick={() => turn("start")}>Start</button>
            <button className="btn btn-warning" onClick={() => turn("wrong")} disabled={!state.started}>
              Wrong ({state.wrong}/{state.wrongLimit})
            </button>
            <button className="btn btn-outline-primary" onClick={() => turn("restart")}>Restart</button>
            <button className="btn btn-outline-danger" onClick={() => turn("end")}>End game</button>
          </div>
        )}
      </div>

      {/* Scoreboard: appears as soon as the first player guesses correctly,
          first-to-guess at the top, growing as more players finish. */}
      {!!state?.scores?.length && (
        <div className="papers-scores p-3 pt-0">
          <div className="card">
            <div className="card-body">
              <h5 className="card-title h6 text-uppercase text-muted mb-3">
                {state.gameOver ? "Final scores" : "Scores so far"}
              </h5>
              <table className="table table-sm mb-0">
                <thead>
                  <tr>
                    <th style={{ width: "10%" }}>#</th>
                    <th>Player</th>
                  </tr>
                </thead>
                <tbody>
                  {state.scores.map((s, i) => (
                    <tr key={s.key} className={i === 0 ? "table-success" : ""}>
                      <td>{i + 1}</td>
                      <td>{s.name || s.key.slice(0, 6)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      )}

      <div className="papers-stage">
        <div className="papers-grid">
          {players.map((pv) => {
            const self = pv.key === selfKey;
            return (
              <Tile
                key={pv.key}
                pv={pv}
                self={self}
                active={pv.active}
                stream={self ? localStream ?? undefined : remoteStreams[pv.key]}
              />
            );
          })}
          {players.length === 0 && <div className="text-muted">Waiting for players…</div>}
        </div>
      </div>

      {/* Live chat, over the same WebRTC mesh used for video (no separate
          backend) — text-only, broadcast to every connected peer. */}
      <div className="papers-chat p-3 pt-0">
        <div className="card">
          <div className="card-body">
            <h5 className="card-title h6 text-uppercase text-muted mb-2">Chat</h5>
            <div
              className="papers-chat-log mb-2"
              style={{ maxHeight: 160, overflowY: "auto" }}
            >
              {chatMessages.length === 0 ? (
                <div className="text-muted small">No messages yet.</div>
              ) : (
                chatMessages.map((m, i) => (
                  <div key={i} className="small">
                    <strong>{m.fromId === selfKey ? "You" : nameFor(m.fromId)}:</strong> {m.text}
                  </div>
                ))
              )}
            </div>
            <div className="d-flex gap-2">
              <input
                className="form-control form-control-sm"
                value={chatInput}
                onChange={(e) => setChatInput(e.target.value)}
                onKeyDown={(e) => { if (e.key === "Enter") sendChat(); }}
                placeholder="Say something…"
              />
              <button className="btn btn-sm btn-primary" onClick={sendChat} disabled={!chatInput.trim()}>
                Send
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};

export default PapersView;
