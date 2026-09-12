import React, { Component } from "react";
import axios from "axios";
import { TextEdit, FloatEdit, SelectEdit, SelectView, CheckboxEdit } from "../index";
import { t as translate, subscribe } from "@engine/i18n";

// TableEdit renders a TYPE_TABLE field as a real sub-fieldset: every column is a
// Field definition (name/type/label/options/readonly) coming from the backend's
// TableFieldset, and each cell is rendered with that column's own editor
// (Select, Checkbox, Date, number, text). Rows are seeded by the module's
// TableData hook (POST /api/modules/{module}/table/{field}); when the field is
// tableRowsAddable the user can also add and remove rows. The value is emitted
// as a JSON array of column-keyed row objects for the backend TableOnSubmit hook.

interface ColumnDef {
  name: string;
  type?: string;
  label?: string;
  readonly?: boolean;
  required?: boolean;
  description?: string;
  default_value?: any;
  options?: Record<string, any>;
}

interface TableFieldMeta {
  name?: string;
  sql?: string;
  tableFieldset?: ColumnDef[];
  tableRowsAddable?: boolean;
  tableRowKey?: string;
  options?: Record<string, any>;
}

interface TableEditProps {
  field?: TableFieldMeta;
  // Flat fallbacks (older call sites / Field.tsx option spread).
  tableFieldset?: ColumnDef[];
  tableRowsAddable?: boolean;
  tableRowKey?: string;
  id?: string;
  fieldName?: string;
  value?: any;
  module?: string;
  formValues?: Record<string, any>;
  disabled?: boolean;
  label?: string;
  onChange?: (value: string) => void;
}

type Row = Record<string, any>;

interface TableEditState {
  serverRows: Row[];
  rows: Row[];
  loading: boolean;
}

const CHECKBOX_TYPES = new Set(["Checkbox", "YesNo", "ActiveInactive"]);
const NUMBER_TYPES = new Set(["Int", "Float", "Money", "MoneyWithCurrency"]);
const DATE_TYPES = new Set(["Date", "DateTime"]);
const SELECT_TYPES = new Set(["Select", "SelectAddNew"]);

// Column name -> bitmask bit, for the "sync a checkbox column from a sibling
// bitmask field" behaviour (field option syncColumnsFromBitmask). Matches
// auth.MODE_* and the rights modules' list/view/create/edit/delete columns.
const BITMASK_BITS: Record<string, number> = {
  list: 1,
  view: 2,
  create: 4,
  edit: 8,
  delete: 16,
};

function truthy(v: any): boolean {
  return v === true || v === 1 || v === "1" || v === "true" || v === "yes";
}

// coerceCell normalizes a cell to its column's native type before emit, so rows
// loaded from the server (numbers) and rows the user picked/typed (strings from
// <select>/<input>) go over the wire as the same type — e.g. an Int/select
// column always emits a number.
function coerceCell(col: ColumnDef, v: any): any {
  if (v === null || v === undefined || v === "") return v;
  const t = col.type || "String";
  if (NUMBER_TYPES.has(t)) {
    const n = typeof v === "number" ? v : Number(v);
    return Number.isFinite(n) ? n : v;
  }
  if (CHECKBOX_TYPES.has(t)) return truthy(v);
  return v;
}

class TableEdit extends Component<TableEditProps, TableEditState> {
  // The field value only becomes the source of truth once the user edits a cell.
  // Before that it may be absent (table not selected) or a display projection
  // (e.g. group *names* resolved by the field's SQL for the view), neither of
  // which is an editable row set — so the initial rows come from TableData.
  private touched = false;

  constructor(props: TableEditProps) {
    super(props);
    this.state = { serverRows: [], rows: [], loading: false };
  }

  private unsubscribeI18n?: () => void;
  componentDidMount() {
    this.loadServerRows();
    this.unsubscribeI18n = subscribe(() => this.forceUpdate());
  }
  componentWillUnmount() {
    this.unsubscribeI18n?.();
  }

  componentDidUpdate(prev: TableEditProps) {
    // Sync a checkbox column from a sibling bitmask field: e.g. checking "list"
    // in "Allowed Modes" checks the "list" cell for every field row (and
    // unchecking clears them). Only reacts to an actual change of that sibling —
    // never on first populate — so stored per-field values aren't wiped on load.
    const sync = this.syncFrom();
    if (sync && sync in (prev.formValues || {})) {
      const before = Number((prev.formValues || {})[sync]) || 0;
      const after = Number((this.props.formValues || {})[sync]) || 0;
      if (before !== after) {
        this.applyBitmaskSync(after);
        return;
      }
    }

    if (this.contextSig(prev) !== this.contextSig()) {
      this.touched = false;
      this.loadServerRows();
    } else if (this.touched && prev.value !== this.props.value) {
      this.setState({ rows: this.computeRows(this.state.serverRows) });
    }
  }

  // syncFrom returns the sibling bitmask field name whose bits drive this table's
  // matching checkbox columns, or "".
  private syncFrom(): string {
    return (
      this.props.field?.options?.syncColumnsFromBitmask ||
      (this.props as any).syncColumnsFromBitmask ||
      ""
    );
  }

  private applyBitmaskSync(mask: number) {
    const cols = this.columns()
      .map((c) => c.name)
      .filter((n) => n in BITMASK_BITS);
    if (!cols.length) return;
    const rows = this.state.rows.map((r) => {
      const nr = { ...r };
      cols.forEach((n) => {
        nr[n] = (mask & BITMASK_BITS[n]) !== 0;
      });
      return nr;
    });
    this.setState({ rows });
    this.emit(rows);
  }

  // --- config ---------------------------------------------------------------

  private columns(): ColumnDef[] {
    return this.props.field?.tableFieldset || this.props.tableFieldset || [];
  }

  private addable(): boolean {
    return !!(this.props.field?.tableRowsAddable ?? this.props.tableRowsAddable);
  }

  private fieldName(): string {
    return (
      this.props.field?.name || this.props.fieldName || this.props.id || ""
    );
  }

  private keyColumn(): string {
    const explicit = this.props.field?.tableRowKey || this.props.tableRowKey;
    if (explicit) return explicit;
    const cols = this.columns();
    const ro = cols.find((c) => c.readonly);
    return (ro || cols[0])?.name || "field";
  }

  // A stable signature of sibling form values (minus this field's own value),
  // so toggling a cell doesn't re-fetch the server rows but changing a sibling
  // select (e.g. the "module" the rights apply to) does.
  private contextSig(props: TableEditProps = this.props): string {
    const fv = props.formValues || {};
    const self = this.fieldName();
    const sync = this.syncFrom(); // handled by applyBitmaskSync, not a reload
    const ctx: Record<string, any> = {};
    Object.keys(fv).forEach((k) => {
      if (k !== self && k !== sync) ctx[k] = fv[k];
    });
    return JSON.stringify(ctx);
  }

  // --- data ---------------------------------------------------------------

  private async loadServerRows() {
    const owner = this.props.module;
    const field = this.fieldName();
    if (!owner || !field) {
      this.setState({ serverRows: [], rows: this.computeRows([]) });
      return;
    }
    this.setState({ loading: true });
    try {
      const res = await axios.post(
        `/api/modules/${owner}/table/${field}`,
        this.props.formValues || {}
      );
      const serverRows: Row[] = ((res.data && res.data.rows) || []).filter(
        (r: any) => r && typeof r === "object"
      );
      this.setState({ serverRows, rows: this.computeRows(serverRows), loading: false });
    } catch {
      this.setState({ serverRows: [], rows: this.computeRows([]), loading: false });
    }
  }

  // computeRows builds the working row set. Until the user edits a cell the rows
  // are the server-seeded rows (from TableData). After that the field value —
  // the array this component emitted — is authoritative.
  private computeRows(serverRows: Row[]): Row[] {
    const key = this.keyColumn();
    const parsed = this.touched ? parseValue(this.props.value, key) : null;

    if (this.addable()) {
      return parsed ? parsed : serverRows.map((r) => ({ ...r }));
    }

    const byKey: Record<string, Row> = {};
    (parsed || []).forEach((r) => {
      byKey[String(r[key])] = r;
    });
    return serverRows.map((sr) => ({ ...sr, ...(byKey[String(sr[key])] || {}) }));
  }

  private blankRow(): Row {
    const row: Row = {};
    this.columns().forEach((c) => {
      row[c.name] = c.default_value ?? (CHECKBOX_TYPES.has(c.type || "") ? false : "");
    });
    return row;
  }

  // --- mutation ---------------------------------------------------------------

  private emit(rows: Row[]) {
    this.touched = true;
    if (!this.props.onChange) return;
    const cols = this.columns();
    const out = rows.map((r) => {
      const o: Row = {};
      cols.forEach((c) => {
        o[c.name] = coerceCell(c, r[c.name]);
      });
      return o;
    });
    this.props.onChange(JSON.stringify(out));
  }

  private setCell = (i: number, name: string, value: any) => {
    const rows = this.state.rows.map((r, idx) => (idx === i ? { ...r, [name]: value } : r));
    this.setState({ rows });
    this.emit(rows);
  };

  private addRow = () => {
    const rows = [...this.state.rows, this.blankRow()];
    this.setState({ rows });
    this.emit(rows);
  };

  private removeRow = (i: number) => {
    const rows = this.state.rows.filter((_, idx) => idx !== i);
    this.setState({ rows });
    this.emit(rows);
  };

  // --- render ---------------------------------------------------------------

  private renderCell(col: ColumnDef, row: Row, i: number) {
    const value = row[col.name];
    const disabled = this.props.disabled || col.readonly;
    const type = col.type || "String";
    const opts = col.options || {};
    const isSelect = opts.widget === "select" || SELECT_TYPES.has(type);
    const onChange = (v: any) => this.setCell(i, col.name, v);

    if (CHECKBOX_TYPES.has(type)) {
      return (
        <CheckboxEdit value={truthy(value)} disabled={disabled} onChange={(v: boolean) => onChange(v)} />
      );
    }

    if (isSelect) {
      const options = [{ name: "—", value: "" }, ...(opts.options || [])];
      if (disabled) {
        return <SelectView value={value} options={opts.options || []} />;
      }
      return <SelectEdit value={value ?? ""} options={options} onChange={(v: any) => onChange(v)} />;
    }

    if (disabled) {
      return <span>{value === undefined || value === null || value === "" ? "—" : String(value)}</span>;
    }

    if (DATE_TYPES.has(type)) {
      return (
        <input
          type={type === "DateTime" ? "datetime-local" : "date"}
          className="form-control"
          value={value ?? ""}
          onChange={(e) => onChange(e.target.value)}
        />
      );
    }
    if (NUMBER_TYPES.has(type)) {
      return <FloatEdit value={value ?? ""} onChange={(v: any) => onChange(v)} />;
    }
    return (
      <TextEdit value={value ?? ""} type="varchar" onChange={(v: string) => onChange(v)} />
    );
  }

  render() {
    const { label } = this.props;
    const cols = this.columns();
    const { rows, loading } = this.state;

    if (!cols.length) {
      return <div className="text-muted">{translate("This table has no columns configured.")}</div>;
    }
    if (loading) {
      return <div className="text-muted">{translate("Loading…")}</div>;
    }

    return (
      <div className="field-table" style={{ maxWidth: "100%", overflowX: "auto" }}>
        {label && <label className="form-label">{label}</label>}
        <table
          className="table table-sm table-bordered align-middle mb-0 w-auto"
          style={{ fontSize: "0.9rem" }}
        >
          <thead>
            <tr>
              {cols.map((c) => (
                <th key={c.name} className="text-nowrap">
                  {c.label ? translate(c.label) : c.name}
                </th>
              ))}
              {this.addable() && <th />}
            </tr>
          </thead>
          <tbody>
            {rows.length === 0 && (
              <tr>
                <td colSpan={cols.length + (this.addable() ? 1 : 0)} className="text-muted">
                  {translate("No rows.")}
                </td>
              </tr>
            )}
            {rows.map((row, i) => (
              <tr key={i}>
                {cols.map((c) => (
                  <td key={c.name}>{this.renderCell(c, row, i)}</td>
                ))}
                {this.addable() && (
                  <td className="text-center">
                    <button
                      type="button"
                      className="btn btn-sm btn-outline-danger"
                      disabled={this.props.disabled}
                      onClick={() => this.removeRow(i)}
                      aria-label={translate("Remove row")}
                    >
                      ×
                    </button>
                  </td>
                )}
              </tr>
            ))}
          </tbody>
        </table>
        {this.addable() && (
          <button
            type="button"
            className="btn btn-sm btn-outline-primary mt-2"
            disabled={this.props.disabled}
            onClick={this.addRow}
          >
            + {translate("Add row")}
          </button>
        )}
      </div>
    );
  }
}

// parseValue normalizes a stored TYPE_TABLE value into a row array:
//   - a row array [{col: val}]                      -> as-is
//   - a scalar array [1, 2] (e.g. stored []int)     -> [{<key>: 1}, {<key>: 2}]
//   - a checkbox map {"title": ["view","edit"]}     -> [{<key>:"title", view:true, edit:true}]
//   - a JSON string of any of the above             -> parsed then normalized
function parseValue(value: any, keyCol: string): Row[] | null {
  let v = value;
  if (v === undefined || v === null || v === "") return null;
  if (typeof v === "string") {
    const s = v.trim();
    if (!s) return null;
    try {
      v = JSON.parse(s);
    } catch {
      return null;
    }
  }

  if (Array.isArray(v)) {
    return v.map((item) =>
      item && typeof item === "object" ? (item as Row) : { [keyCol]: item }
    );
  }

  if (v && typeof v === "object") {
    return Object.keys(v).map((k) => {
      const row: Row = { [keyCol]: k };
      const arr = (v as Record<string, any>)[k];
      if (Array.isArray(arr)) arr.forEach((c) => (row[String(c)] = true));
      return row;
    });
  }

  return null;
}

export default TableEdit;
