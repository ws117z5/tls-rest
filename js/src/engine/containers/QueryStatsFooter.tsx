import React, { useEffect, useState } from "react";
import QueryStats, { QueryStatsSnapshot } from "@engine/containers/QueryStatsBus";

// QueryStatsFooter shows the combined DB query count/duration of the most
// recent click (which may have fanned out into several requests — see
// QueryStatsBus.ts), admin sessions only. Mount once at the app root.
const QueryStatsFooter: React.FC = () => {
  const [snap, setSnap] = useState<QueryStatsSnapshot | null>(null);

  useEffect(() => QueryStats.subscribe(setSnap), []);

  if (!snap) return null;

  return (
    <div style={barStyle}>
      {snap.count} {snap.count === 1 ? "query" : "queries"} · {snap.ms.toFixed(1)} ms · {snap.paths.join(", ")}
    </div>
  );
};

const barStyle: React.CSSProperties = {
  position: "fixed",
  bottom: 0,
  left: 0,
  right: 0,
  zIndex: 2000,
  padding: "4px 12px",
  background: "#1e1e1e",
  color: "#0f0",
  fontSize: 12,
  fontFamily: "monospace",
  textAlign: "right",
};

export default QueryStatsFooter;
