import React, { useCallback, useEffect, useRef, useState } from "react";
import axios from "axios";
import { ModuleViewProps } from "@engine/controllers/registry";
import "./papers.css";

interface PlayerView { key: string; name: string; word?: string; ready: boolean; active: boolean; }
interface GameState {
  selfName: string; selfWord: string; isCreator: boolean;
  players: PlayerView[]; activeKey: string; started: boolean;
  wrong: number; wrongLimit: number; timerSecs: number; remaining: number;
  deadline: number; selfKey: string;
}

// A square video tile. self => local camera. The assigned word shows ABOVE the
// tile for everyone except the player wearing it.
const Tile: React.FC<{ pv: PlayerView; self: boolean; active: boolean }> = ({ pv, self, active }) => {
  const ref = useRef<HTMLVideoElement>(null);
  useEffect(() => {
    if (!self) return;
    let stream: MediaStream | null = null;
    navigator.mediaDevices.getUserMedia({ video: true, audio: false })
      .then((s) => { stream = s; if (ref.current) ref.current.srcObject = s; })
      .catch(() => {});
    return () => { stream?.getTracks().forEach((t) => t.stop()); };
  }, [self]);
  return (
    <div className={`papers-tile ${active ? "active" : ""} ${self ? "you" : ""}`}>
      {!self && pv.word ? <div className="papers-word">{pv.word}</div> : null}
      <video id={pv.key} ref={self ? ref : undefined} className="papers-video" autoPlay playsInline muted={self} />
      <span className="papers-name">{pv.name || pv.key.slice(0, 6)}{active ? " ⏳" : ""}</span>
    </div>
  );
};

const PapersView: React.FC<ModuleViewProps> = ({ record, module, navigate }) => {
  const room = record || {};
  const roomId = room.uuid || room.id;
  const base = `/papers/${roomId}/game`;

  const [state, setState] = useState<GameState | null>(null);
  const [name, setName] = useState("");
  const [word, setWord] = useState("");
  const [countdown, setCountdown] = useState(0);

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

  // Subscribe to live updates (SSE): the server pushes a "changed" event on any
  // state change (join / start / wrong / turn expiry) and we pull our own state.
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
    es.onmessage = () => refresh();       // "changed" pings
    // EventSource auto-reconnects on error; a slow fallback poll covers gaps.
    const fallback = setInterval(refresh, 10000);
    return () => { done = true; es.close(); clearInterval(fallback); };
  }, [refresh, base]);

  // Local 1s countdown, re-synced from server `remaining` on each poll.
  useEffect(() => {
    if (!state?.started) { setCountdown(0); return; }
    setCountdown(state.remaining);
    const id = setInterval(() => setCountdown((c) => Math.max(0, c - 1)), 1000);
    return () => clearInterval(id);
  }, [state?.started, state?.remaining, state?.activeKey]);

  const join = async () => {
    await axios.post(`${base}/join`, { name: name.trim(), word: word.trim() });
    refresh();
  };
  const turn = async (action: string) => {
    await axios.post(`${base}/turn`, { action });
    if (action === "end") { navigate(`/${module}`); return; }
    refresh();
  };

  const players = state?.players ?? [];
  const selfKey = state?.selfKey;
  const mmss = (s: number) => `${String(Math.floor(s / 60)).padStart(2, "0")}:${String(s % 60).padStart(2, "0")}`;

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
          {state?.started && <span className="papers-timer">{mmss(countdown)}</span>}
          <button className="btn btn-sm btn-secondary" onClick={() => navigate(`/${module}`)}>Leave</button>
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

      <div className="papers-stage">
        <div className="papers-grid">
          {players.map((pv) => (
            <Tile key={pv.key} pv={pv} self={pv.key === selfKey} active={pv.active} />
          ))}
          {players.length === 0 && <div className="text-muted">Waiting for players…</div>}
        </div>
      </div>
    </div>
  );
};

export default PapersView;