import React, { useEffect, useState } from "react";
import Notify, { Notification } from "@engine/containers/Notify";

// Toasts subscribes to the Notify bus and renders a stack of dismissible
// notifications in the top-right corner. Mount once at the app root.
const Toasts: React.FC = () => {
  const [items, setItems] = useState<Notification[]>([]);

  useEffect(() => {
    return Notify.subscribe((n) => {
      setItems((prev) => [...prev, n]);
      if (n.timeout && n.timeout > 0) {
        window.setTimeout(() => {
          setItems((prev) => prev.filter((i) => i.id !== n.id));
        }, n.timeout);
      }
    });
  }, []);

  const dismiss = (id: number) => setItems((prev) => prev.filter((i) => i.id !== id));

  if (items.length === 0) return null;

  return (
    <div style={stackStyle} aria-live="polite">
      {items.map((n) => (
        <div key={n.id} style={{ ...itemStyle, ...kindStyle(n.kind) }} role="alert">
          <div style={{ flex: 1 }}>
            <div>{n.message}</div>
            {n.logId && <div style={logStyle}>log id: {n.logId}</div>}
          </div>
          <button onClick={() => dismiss(n.id)} style={closeStyle} aria-label="Dismiss">
            ×
          </button>
        </div>
      ))}
    </div>
  );
};

const stackStyle: React.CSSProperties = {
  position: "fixed",
  top: 16,
  right: 16,
  zIndex: 2000,
  display: "flex",
  flexDirection: "column",
  gap: 8,
  maxWidth: 380,
};

const itemStyle: React.CSSProperties = {
  display: "flex",
  alignItems: "flex-start",
  gap: 8,
  padding: "10px 12px",
  borderRadius: 6,
  boxShadow: "0 2px 8px rgba(0,0,0,0.2)",
  color: "#fff",
  fontSize: 14,
};

const logStyle: React.CSSProperties = { fontSize: 11, opacity: 0.85, marginTop: 4, fontFamily: "monospace" };

const closeStyle: React.CSSProperties = {
  background: "transparent",
  border: "none",
  color: "inherit",
  fontSize: 18,
  lineHeight: 1,
  cursor: "pointer",
  padding: 0,
};

function kindStyle(kind: Notification["kind"]): React.CSSProperties {
  switch (kind) {
    case "error":
      return { background: "#c0392b" };
    case "success":
      return { background: "#2e7d32" };
    case "warning":
      return { background: "#b8860b" };
    default:
      return { background: "#34495e" };
  }
}

export default Toasts;