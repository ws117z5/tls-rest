import React from "react";

// TableView renders a TYPE_TABLE field read-only. It accepts every shape the
// backend may hand back:
//   - a scalar array (e.g. group names resolved by an SQL subselect) -> chips
//   - a row array [{col: val}] with column defs -> a table
//   - a checkbox map {"title": ["view","edit"]} -> "field: modes" list
//   - a JSON string of any of the above

interface ColumnDef {
  name: string;
  label?: string;
  type?: string;
}

interface TableFieldMeta {
  tableFieldset?: ColumnDef[];
}

interface TableViewProps {
  field?: TableFieldMeta;
  tableFieldset?: ColumnDef[];
  value?: any;
}

function parse(value: any): any {
  if (typeof value !== "string") return value;
  const s = value.trim();
  if (!s) return null;
  try {
    return JSON.parse(s);
  } catch {
    return s; // legacy CSV / plain text
  }
}

const TableView: React.FC<TableViewProps> = ({ field, tableFieldset, value }) => {
  const v = parse(value);
  const columns = field?.tableFieldset || tableFieldset || [];

  if (v === null || v === undefined || v === "") {
    return <span className="text-muted">—</span>;
  }

  if (typeof v === "string") {
    return <span>{v}</span>;
  }

  if (Array.isArray(v)) {
    if (v.length === 0) return <span className="text-muted">—</span>;

    // Scalar array (names / ids): render as chips.
    if (v.every((x) => x === null || typeof x !== "object")) {
      return (
        <span className="d-inline-flex flex-wrap gap-1">
          {v.map((x, i) => (
            <span key={i} className="badge bg-secondary-subtle text-body">
              {String(x)}
            </span>
          ))}
        </span>
      );
    }

    // Row array: render a table using the column defs when we have them,
    // otherwise the union of keys present in the rows.
    const cols: ColumnDef[] = columns.length
      ? columns
      : Array.from(
          v.reduce((set: Set<string>, r: any) => {
            Object.keys(r || {}).forEach((k) => set.add(k));
            return set;
          }, new Set<string>())
        ).map((name) => ({ name }));

    return (
      <div style={{ maxWidth: "100%", overflowX: "auto" }}>
        <table className="table table-sm table-bordered mb-0 w-auto" style={{ fontSize: "0.9rem" }}>
          <thead>
            <tr>
              {cols.map((c) => (
                <th key={c.name} className="text-nowrap">
                  {c.label || c.name}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {v.map((row: any, i: number) => (
              <tr key={i}>
                {cols.map((c) => {
                  const cell = row ? row[c.name] : undefined;
                  return (
                    <td key={c.name}>
                      {typeof cell === "boolean"
                        ? cell
                          ? "✓"
                          : ""
                        : cell === undefined || cell === null || cell === ""
                          ? ""
                          : String(cell)}
                    </td>
                  );
                })}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    );
  }

  if (typeof v === "object") {
    const keys = Object.keys(v);
    if (keys.length === 0) return <span className="text-muted">All fields</span>;
    return (
      <ul className="list-unstyled mb-0">
        {keys.map((f) => (
          <li key={f}>
            <span className="fw-medium">{f}</span>:{" "}
            <span className="text-muted">
              {Array.isArray(v[f]) && v[f].length ? v[f].join(", ") : "denied"}
            </span>
          </li>
        ))}
      </ul>
    );
  }

  return <span>{String(v)}</span>;
};

export default TableView;
