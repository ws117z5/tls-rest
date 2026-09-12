import React, { Component } from "react";
import axios from "axios";
import { t, subscribe } from "@engine/i18n";

// TableView renders a TYPE_TABLE field read-only.
//
//   - When the field has an inline value (e.g. group names resolved by an SQL
//     subselect) it is rendered directly: a scalar array as chips, a row array
//     as a table, a {field:[modes]} map as a list.
//   - When there is no inline value but the field is server-backed (module +
//     field name known, e.g. an attached comments thread), the rows are fetched
//     from /api/modules/{module}/table/{field} and shown read-only.

interface ColumnDef {
  name: string;
  label?: string;
  type?: string;
  options?: any;
}

// cellText renders one read-only cell. A select column's stored value (an id) is
// resolved to its option label so the view shows names, not ids.
function cellText(col: ColumnDef, cell: any): string {
  if (typeof cell === "boolean") return cell ? "✓" : "";
  if (cell === undefined || cell === null || cell === "") return "";
  const opts = col.options && col.options.options;
  if (Array.isArray(opts)) {
    const hit = opts.find((o: any) => String(o && o.value !== undefined ? o.value : o) === String(cell));
    if (hit) return String(hit.name ?? hit.label ?? hit.value ?? cell);
  }
  return String(cell);
}

interface FieldMeta {
  name?: string;
  tableFieldset?: ColumnDef[];
}

interface TableViewProps {
  field?: FieldMeta;
  tableFieldset?: ColumnDef[];
  module?: string;
  formValues?: Record<string, any>;
  value?: any;
}

interface TableViewState {
  rows: any[] | null; // null = not fetched
  loading: boolean;
}

class TableView extends Component<TableViewProps, TableViewState> {
  state: TableViewState = { rows: null, loading: false };

  private unsubscribeI18n?: () => void;
  componentDidMount() {
    if (this.shouldFetch()) this.load();
    this.unsubscribeI18n = subscribe(() => this.forceUpdate());
  }
  componentWillUnmount() {
    this.unsubscribeI18n?.();
  }

  private columns(): ColumnDef[] {
    return this.props.field?.tableFieldset || this.props.tableFieldset || [];
  }

  private fieldName(): string {
    return this.props.field?.name || "";
  }

  private shouldFetch(): boolean {
    const v = this.props.value;
    const hasValue = v !== undefined && v !== null && v !== "";
    return !hasValue && !!this.props.module && !!this.fieldName();
  }

  private async load() {
    this.setState({ loading: true });
    try {
      const res = await axios.post(
        `/api/modules/${this.props.module}/table/${this.fieldName()}`,
        this.props.formValues || {}
      );
      const rows = (res.data && res.data.rows) || [];
      this.setState({ rows: Array.isArray(rows) ? rows : [], loading: false });
    } catch {
      this.setState({ rows: [], loading: false });
    }
  }

  render() {
    if (this.state.rows !== null) {
      return renderValue(this.state.rows, this.columns());
    }
    if (this.state.loading) {
      return <span className="text-muted">…</span>;
    }
    return renderValue(this.props.value, this.columns());
  }
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

function renderValue(value: any, columns: ColumnDef[]): React.ReactElement {
  const v = parse(value);

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
                  {c.label ? t(c.label) : c.name}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {v.map((row: any, i: number) => (
              <tr key={i}>
                {cols.map((c) => (
                  <td key={c.name}>{cellText(c, row ? row[c.name] : undefined)}</td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    );
  }

  if (typeof v === "object") {
    const keys = Object.keys(v);
    if (keys.length === 0) return <span className="text-muted">{t("All fields")}</span>;
    return (
      <ul className="list-unstyled mb-0">
        {keys.map((f) => (
          <li key={f}>
            <span className="fw-medium">{f}</span>:{" "}
            <span className="text-muted">
              {Array.isArray(v[f]) && v[f].length ? v[f].join(", ") : t("denied")}
            </span>
          </li>
        ))}
      </ul>
    );
  }

  return <span>{String(v)}</span>;
}

export default TableView;
