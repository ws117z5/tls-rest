import React from "react";
import { ModuleViewProps } from "@engine/controllers/registry";

// Papers list = all active games (rooms). Each row opens the room's view.
function playerCount(room: any): number {
  const u = room?.users;
  if (Array.isArray(u)) return u.length;
  if (typeof u === "string" && u.trim()) {
    try { const a = JSON.parse(u); if (Array.isArray(a)) return a.length; } catch { /* */ }
  }
  return 0;
}

const PapersList: React.FC<ModuleViewProps> = ({ data, navigate, module }) => {
  const rooms = Array.isArray(data) ? data : [];
  if (rooms.length === 0) {
    return <div className="text-muted p-4 text-center">No active games right now.</div>;
  }
  return (
    <div className="papers-list d-flex flex-column gap-2 p-2">
      {rooms.map((room: any) => {
        // Rooms are addressed by their stored hash (KeyField), never uuid or id.
        const key = room.hash;
        const n = playerCount(room);
        return (
          <div
            key={key}
            className="card"
            style={{ cursor: "pointer" }}
            onClick={() => navigate(`/${module}/${key}`)}
          >
            <div className="card-body d-flex justify-content-between align-items-center py-2">
              <div>
                <h5 className="mb-0">{room.name || `Room ${room.id}`}</h5>
                <small className="text-muted">{n} player{n === 1 ? "" : "s"}</small>
              </div>
              <span className="badge bg-success">Join</span>
            </div>
          </div>
        );
      })}
    </div>
  );
};

export default PapersList;