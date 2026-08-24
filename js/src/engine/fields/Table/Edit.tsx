import React, { Component } from "react";
import axios from "axios";

// TableEdit renders a table whose columns come from the field's TableFieldset
// (a read-only key column plus one checkbox column per mode) and whose rows are
// provided by the server: it POSTs the current form values to
// /api/modules/{module}/table/{field}, which runs the field's TableData hook.
// The value is submitted as rows [{field, list:bool, ...}] for TableOnSubmit.
interface ColumnDef {
  name: string;
  type?: string;
  label?: string;
  readonly?: boolean;
}

interface TableEditProps {
  value?: any;
  tableFieldset?: ColumnDef[]; // column definitions (clean API)
  columns?: string[]; // legacy: mode column names
  module?: string;    // owning module (rights module) — for the table endpoint
  fieldName?: string; // this field's name — for the table endpoint
  id?: string;
  formValues?: Record<string, any>;
  disabled?: boolean;
  label?: string;
  onChange?: (value: string) => void;
}

interface TableEditState {
  rows: string[]; // field names of the target module
  matrix: Record<string, Set<string>>; // field -> set of allowed modes
  loading: boolean;
}

const DEFAULT_COLUMNS = ["list", "view", "create", "edit", "delete"];

class TableEdit extends Component<TableEditProps, TableEditState> {
  constructor(props: TableEditProps) {
    super(props);
    this.state = { rows: [], matrix: parseValue(props.value), loading: false };
  }

  componentDidMount() {
    this.loadRows();
  }

  componentDidUpdate(prev: TableEditProps) {
    if (prev.value !== this.props.value) {
      this.setState({ matrix: parseValue(this.props.value) });
    }
    // Refetch rows whenever the context changes (excluding this field's own
    // value, so editing checkboxes doesn't re-trigger a load).
    if (this.contextSig(prev) !== this.contextSig()) {
      this.loadRows();
    }
  }

  // A stable signature of the sibling form values (minus this field's own value),
  // used to decide when to re-request the server-provided rows.
  private contextSig(props: TableEditProps = this.props): string {
    const fv = props.formValues || {};
    const self = props.fieldName || props.id || "";
    const ctx: Record<string, any> = {};
    Object.keys(fv).forEach((k) => {
      if (k !== self) ctx[k] = fv[k];
    });
    return JSON.stringify(ctx);
  }

  // The "key" column (read-only, holds the row identifier — e.g. the field
  // name) and the toggle (mode) columns are derived from the table fieldset.
  private keyColumn(): string {
    const fs = this.props.tableFieldset;
    if (fs && fs.length) {
      const ro = fs.find((c) => c.readonly);
      return (ro || fs[0]).name;
    }
    return "field";
  }

  private columns(): string[] {
    const fs = this.props.tableFieldset;
    if (fs && fs.length) {
      const key = this.keyColumn();
      return fs.filter((c) => c.name !== key).map((c) => c.name);
    }
    return this.props.columns && this.props.columns.length ? this.props.columns : DEFAULT_COLUMNS;
  }

  private columnLabel(name: string): string {
    const fs = this.props.tableFieldset;
    const c = fs && fs.find((x) => x.name === name);
    return (c && c.label) || name;
  }

  private async loadRows() {
    const owner = this.props.module;
    const field = this.props.fieldName || this.props.id;
    if (!owner || !field) {
      this.setState({ rows: [] });
      return;
    }
    this.setState({ loading: true });
    try {
      // Server runs the field's TableData(ctx) hook and returns the rows.
      const res = await axios.post(
        `/api/modules/${owner}/table/${field}`,
        this.props.formValues || {}
      );
      const key = this.keyColumn();
      const rows = ((res.data && res.data.rows) || [])
        .map((r: any) => (r && (r[key] ?? r.field)))
        .filter((n: any) => typeof n === "string" && n);
      this.setState({ rows, loading: false });
    } catch {
      this.setState({ rows: [], loading: false });
    }
  }

  private emit(matrix: Record<string, Set<string>>) {
    if (!this.props.onChange) return;
    // Emit an array of row objects — {<key>: name, <mode>: bool, ...} — so the
    // backend TableOnSubmit hook can process them. Only rows currently shown are
    // included.
    const cols = this.columns();
    const key = this.keyColumn();
    const rows = this.state.rows.map((f) => {
      const set = matrix[f] || new Set<string>();
      const row: Record<string, any> = { [key]: f };
      cols.forEach((c) => {
        row[c] = set.has(c);
      });
      return row;
    });
    this.props.onChange(JSON.stringify(rows));
  }

  private toggle(field: string, mode: string, on: boolean) {
    const matrix = { ...this.state.matrix, [field]: new Set(this.state.matrix[field] || []) };
    if (on) matrix[field].add(mode);
    else matrix[field].delete(mode);
    this.setState({ matrix });
    this.emit(matrix);
  }

  private toggleAll(field: string, on: boolean) {
    const matrix = { ...this.state.matrix, [field]: new Set(on ? this.columns() : []) };
    this.setState({ matrix });
    this.emit(matrix);
  }

  render() {
    const { disabled, label } = this.props;
    const cols = this.columns();

    if (this.state.loading) {
      return <div className="text-muted">Loading fields…</div>;
    }
    if (this.state.rows.length === 0) {
      return <div className="text-muted">Select a module to configure field access.</div>;
    }

    return (
      <div className="field-table" style={{ maxWidth: "100%", overflowX: "auto" }}>
        {label && <label className="form-label">{label}</label>}
        <table className="table table-sm table-bordered align-middle mb-0 w-auto" style={{ fontSize: "0.9rem" }}>
          <thead>
            <tr>
              <th>{this.columnLabel(this.keyColumn()) || "Field"}</th>
              {cols.map((c) => (
                <th key={c} className="text-center text-capitalize">{this.columnLabel(c)}</th>
              ))}
              <th className="text-center">all</th>
            </tr>
          </thead>
          <tbody>
            {this.state.rows.map((f) => {
              const set = this.state.matrix[f] || new Set<string>();
              const allOn = cols.every((c) => set.has(c));
              return (
                <tr key={f}>
                  <td className="fw-medium">{f}</td>
                  {cols.map((c) => (
                    <td key={c} className="text-center">
                      <input
                        type="checkbox"
                        className="form-check-input"
                        checked={set.has(c)}
                        disabled={disabled}
                        onChange={(e) => this.toggle(f, c, e.target.checked)}
                      />
                    </td>
                  ))}
                  <td className="text-center">
                    <input
                      type="checkbox"
                      className="form-check-input"
                      checked={allOn}
                      disabled={disabled}
                      onChange={(e) => this.toggleAll(f, e.target.checked)}
                    />
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
        <small className="text-muted">Leave every box unchecked for a field to deny it; leave the whole table empty to allow all fields.</small>
      </div>
    );
  }
}

// parseValue accepts a JSON string / object of {field: [modes]} and builds the
// field -> Set(modes) matrix. Tolerates a legacy CSV of allowed field names
// (each field granted all listed modes is left to the backend; here we just
// seed the keys).
function parseValue(value: any): Record<string, Set<string>> {
  const matrix: Record<string, Set<string>> = {};
  if (!value) return matrix;
  let v = value;
  if (typeof v === "string") {
    const s = v.trim();
    if (!s) return matrix;
    try {
      v = JSON.parse(s);
    } catch {
      // legacy CSV: names only
      s.split(",").forEach((name) => {
        const n = name.trim();
        if (n) matrix[n] = new Set(DEFAULT_COLUMNS);
      });
      return matrix;
    }
  }
  // Rows array: [{ field: "title", view: true, edit: false, ... }]
  if (Array.isArray(v)) {
    v.forEach((row) => {
      if (!row || typeof row !== "object") return;
      const name = row.field || row[Object.keys(row)[0]];
      if (!name) return;
      const set = new Set<string>();
      Object.keys(row).forEach((k) => {
        if (k !== "field" && (row[k] === true || row[k] === "true" || row[k] === 1)) set.add(k);
      });
      matrix[String(name)] = set;
    });
    return matrix;
  }
  // Stored map: { "title": ["view","edit"] }
  if (v && typeof v === "object") {
    Object.keys(v).forEach((f) => {
      matrix[f] = new Set(Array.isArray(v[f]) ? v[f] : []);
    });
  }
  return matrix;
}

export default TableEdit;