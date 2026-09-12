import React, { useCallback, useEffect, useRef, useState } from "react";
import axios from "axios";
import { ModuleViewProps } from "@engine/controllers/registry";
import { RoomMesh } from "./videoMesh";
import useT from "@engine/useT";
import "./papers.css";

interface PlayerView { key: string; name: string; word?: string; ready: boolean; active: boolean; finished: boolean; }
interface ScoreEntry { key: string; name: string; }
interface GameState {
  selfName: string; selfWord: string; selfFinished: boolean; isCreator: boolean;
  players: PlayerView[]; scores: ScoreEntry[]; gameOver: boolean;
  activeKey: string; started: boolean; roundStarted: boolean;
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
  const t = useT();
  const room = record || {};
  // Rooms are addressed by their stored hash (KeyField), never uuid or id.
  const roomId = room.hash;
  const base = `/papers/${roomId}/game`;

  const [state, setState] = useState<GameState | null>(null);
  const [name, setName] = useState("");
  const [word, setWord] = useState("");
  const [countdown, setCountdown] = useState(0);
  // Name and word each start as an editable input with their own Set button;
  // clicking Set locks that field into a read-only view (with its own Edit
  // button to reopen it). Also flipped to the view on first load when the
  // server already has that field (e.g. the page was reloaded after an
  // earlier Set).
  const [editingName, setEditingName] = useState(true);
  const [editingWord, setEditingWord] = useState(true);

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
      await axios.post(`/papers/${roomId}`, { uuid: "self", name: name.trim() || t("player"), password });
      setJoined(true);
    } catch (e: any) {
      if (e?.response?.status === 403) {
        navigate(`/${module}`); // wrong password -> back to the list
      } else {
        setPwError(t("Could not join the room."));
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
        if (s) {
          if (!name && s.selfName) setName(s.selfName);
          if (!word && s.selfWord) setWord(s.selfWord);
          if (s.selfName) setEditingName(false);
          if (s.selfWord) setEditingWord(false);
        }
        return s;
      });
    });
    const es = new EventSource(`${base}/events`);
    es.onmessage = () => { refresh(); drainSignals(); };
    // EventSource auto-reconnects on error; a slow fallback poll covers gaps.
    const fallback = setInterval(() => { refresh(); drainSignals(); }, 10000);
    return () => { done = true; es.close(); clearInterval(fallback); };
  }, [refresh, drainSignals, base]);

  // Word locking: once the creator's first Start deranges the words
  // (roundStarted), your word is fixed for the rest of the round — the view
  // render below hides the Edit button while roundStarted is true. When a
  // new round begins (restart clears roundStarted and selfWord together),
  // flip back to the input so a word is required again before the next Start.
  useEffect(() => {
    if (state && !state.roundStarted && !state.selfWord) setEditingWord(true);
  }, [state?.roundStarted, state?.selfWord]);

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

  // Name and word are set independently, but both go through the same /join
  // endpoint (it replaces the whole identity each call), so each submit sends
  // the other field's current value along unchanged.
  const setNameField = async () => {
    await axios.post(`${base}/join`, { name: name.trim(), word: word.trim() });
    setEditingName(false);
    refresh();
  };
  const setWordField = async () => {
    await axios.post(`${base}/join`, { name: name.trim(), word: word.trim() });
    setEditingWord(false);
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
          <h4 className="mb-0">{room.name || t("Room")}</h4>
        </div>
        <div className="papers-modal-backdrop">
          <div className="papers-modal card shadow">
            <div className="card-body">
              <h5 className="card-title mb-3">{t("Password required")}</h5>
              <p className="text-muted small mb-2">{t("This game is password-protected.")}</p>
              <label className="form-label mb-1">{t("Your name")}</label>
              <input className="form-control mb-2" value={name} onChange={(e) => setName(e.target.value)} />
              <label className="form-label mb-1">{t("Password")}</label>
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
                <button className="btn btn-secondary" onClick={() => navigate(`/${module}`)}>{t("Cancel")}</button>
                <button className="btn btn-primary" onClick={attemptJoin} disabled={!password}>{t("Enter")}</button>
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
        <h4 className="mb-0">{room.name || t("Room")}</h4>
        <div className="d-flex align-items-center gap-3">
          {/* No in-room Leave button: the page chrome's Back button (above) is
              the one way out, and it unmounts this view, which stops the
              camera and closes every peer connection. */}
          {state?.started && <span className="papers-timer">{mmss(countdown)}</span>}
        </div>
      </div>

      {/* Name + word entry (prefilled). Word is assigned to another player. */}
      <div className="papers-controls d-flex flex-wrap gap-2 align-items-end p-3">
        {editingName ? (
          <div className="d-flex gap-2 align-items-end">
            <div>
              <label className="form-label mb-1">{t("Your name")}</label>
              <input className="form-control" style={{ maxWidth: 180 }} value={name} onChange={(e) => setName(e.target.value)} />
            </div>
            <button className="btn btn-primary" onClick={setNameField} disabled={!name.trim()}>{t("Set")}</button>
          </div>
        ) : (
          <div className="d-flex align-items-center gap-2">
            <span><strong>{name}</strong></span>
            <button className="btn btn-sm btn-outline-secondary" onClick={() => setEditingName(true)}>{t("Edit")}</button>
          </div>
        )}

        {editingWord && !state?.roundStarted ? (
          <div className="d-flex gap-2 align-items-end">
            <div>
              <label className="form-label mb-1">{t("Your word")}</label>
              <input className="form-control" style={{ maxWidth: 200 }} value={word} onChange={(e) => setWord(e.target.value)}
                     placeholder={t("for someone else")} />
            </div>
            <button className="btn btn-primary" onClick={setWordField} disabled={!word.trim()}>{t("Set")}</button>
          </div>
        ) : (
          <div className="d-flex align-items-center gap-2">
            <span className="text-muted small">{t("Word set")} ✓</span>
            {/* No Edit once the round's words are deranged — changing it now
                would just be pointless, someone else already wears it. */}
            {!state?.roundStarted && (
              <button className="btn btn-sm btn-outline-secondary" onClick={() => setEditingWord(true)}>{t("Edit")}</button>
            )}
          </div>
        )}

        {/* Creator-only game controls. */}
        {state?.isCreator && (
          <div className="ml-auto d-flex gap-2">
            {/* Starting alone would derange nobody's word onto anybody else's
                tile — need at least one other player in the room. */}
            {players.length > 1 && (
              <button className="btn btn-success" onClick={() => turn("start")}>{t("Start")}</button>
            )}
            {/* Creator confirms over video that the active player said their
                word correctly — only meaningful while a turn is running. */}
            {state.started && (
              <button className="btn btn-outline-success" onClick={() => turn("guessed")}>{t("Word guessed")}</button>
            )}
            <button className="btn btn-warning" onClick={() => turn("wrong")} disabled={!state.started}>
              {t("Wrong")} ({state.wrong}/{state.wrongLimit})
            </button>
            <button className="btn btn-outline-primary" onClick={() => turn("restart")}>{t("Restart")}</button>
            <button className="btn btn-outline-danger" onClick={() => turn("end")}>{t("End game")}</button>
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
                {state.gameOver ? t("Final scores") : t("Scores so far")}
              </h5>
              <table className="table table-sm mb-0">
                <thead>
                  <tr>
                    <th style={{ width: "10%" }}>#</th>
                    <th>{t("Player")}</th>
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
          {players.length === 0 && <div className="text-muted">{t("Waiting for players…")}</div>}
        </div>
      </div>

      {/* Live chat, over the same WebRTC mesh used for video (no separate
          backend) — text-only, broadcast to every connected peer. */}
      <div className="papers-chat p-3 pt-0">
        <div className="card">
          <div className="card-body">
            <h5 className="card-title h6 text-uppercase text-muted mb-2">{t("Chat")}</h5>
            <div
              className="papers-chat-log mb-2"
              style={{ maxHeight: 160, overflowY: "auto" }}
            >
              {chatMessages.length === 0 ? (
                <div className="text-muted small">{t("No messages yet.")}</div>
              ) : (
                chatMessages.map((m, i) => (
                  <div key={i} className="small">
                    <strong>{m.fromId === selfKey ? t("You") : nameFor(m.fromId)}:</strong> {m.text}
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
                placeholder={t("Say something…")}
              />
              <button className="btn btn-sm btn-primary" onClick={sendChat} disabled={!chatInput.trim()}>
                {t("Send")}
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};

export default PapersView;
