import React, { useEffect, useRef, useState } from "react";
import { MeshGame, Signaling } from "../controllers/meshgame";
import { GameView } from "../controllers/game";

// GameBoard renders the papers game: you set your name + word (you never see the
// word assigned to YOU), and you see the words assigned to everyone else.
const GameBoard: React.FC<{ signaling: Signaling }> = ({ signaling }) => {
  const gameRef = useRef<MeshGame | null>(null);
  const [view, setView] = useState<GameView | null>(null);
  const [name, setName] = useState("");
  const [word, setWord] = useState("");

  useEffect(() => {
    const g = new MeshGame(signaling);
    gameRef.current = g;
    g.onView(setView);
    return () => {
      // MeshManager peers close on GC; add explicit teardown if you expose one.
    };
  }, [signaling]);

  const submit = () => gameRef.current?.setSelf(name.trim(), word.trim());

  const players = view ? [...view.players.values()] : [];

  return (
    <div className="papers-game p-3">
      <div className="mb-3" style={{ maxWidth: 420 }}>
        <label className="form-label">Your name</label>
        <input className="form-control mb-2" value={name} onChange={(e) => setName(e.target.value)} />
        <label className="form-label">Your word (assigned to someone else)</label>
        <input className="form-control mb-2" value={word} onChange={(e) => setWord(e.target.value)} />
        <button className="btn btn-primary" onClick={submit} disabled={!name.trim() || !word.trim()}>
          Set
        </button>
      </div>

      <div className="mb-2 text-muted small">
        {view?.myWordHidden
          ? "Your word is on your forehead — others can see it, you can't."
          : "Waiting for everyone to submit a word…"}
      </div>

      <h6>Words you can see</h6>
      {view && view.visible.length > 0 ? (
        <ul className="list-group" style={{ maxWidth: 480 }}>
          {view.visible.map((a) => {
            const who = view.players.get(a.forClient);
            return (
              <li key={a.forClient} className="list-group-item d-flex justify-content-between">
                <span>{who?.name || a.forClient}</span>
                <strong>{a.word}</strong>
              </li>
            );
          })}
        </ul>
      ) : (
        <div className="text-muted small">No assignments yet.</div>
      )}

      <div className="mt-3 text-muted small">
        Players: {players.map((p) => p.name || p.id).join(", ") || "—"}
      </div>
    </div>
  );
};

export default GameBoard;