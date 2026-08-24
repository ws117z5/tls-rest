import React from "react";

// TableView renders the stored per-field access map read-only:  "field: mode, mode".
interface TableViewProps {
  value?: any;
}

const TableView: React.FC<TableViewProps> = ({ value }) => {
  let v = value;
  if (typeof v === "string") {
    const s = v.trim();
    if (!s) return <span className="text-muted">All fields</span>;
    try {
      v = JSON.parse(s);
    } catch {
      return <span>{s}</span>; // legacy CSV
    }
  }
  if (!v || typeof v !== "object" || Object.keys(v).length === 0) {
    return <span className="text-muted">All fields</span>;
  }
  return (
    <ul className="list-unstyled mb-0">
      {Object.keys(v).map((f) => (
        <li key={f}>
          <span className="fw-medium">{f}</span>:{" "}
          <span className="text-muted">
            {Array.isArray(v[f]) && v[f].length ? v[f].join(", ") : "denied"}
          </span>
        </li>
      ))}
    </ul>
  );
};

export default TableView;