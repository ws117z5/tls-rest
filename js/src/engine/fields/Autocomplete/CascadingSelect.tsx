import React, { Component } from "react";
import axios from "axios";
import { AutoOption } from "./shared";

// CascadingSelect renders a field whose options come from the full-list
// autocomplete endpoint (not the type-ahead one) as a real <select> — used
// when a field's options depend on the record being edited rather than the
// viewer's own authority (WithOptionsCtx only exposes the latter). POSTs
// {input: "", values: context} to /api/modules/{owner}/autocomplete/{field}
// and re-fetches whenever any OTHER key in `context` changes.
//
// Same fetch/render shape as Table/Edit.tsx's CascadingSelectCell, which
// predates this and is left as its own copy (a table cell's "context" is its
// row, not a whole record) rather than risk refactoring an already-working
// component to share this one.
interface CascadingSelectProps {
  owner: string;
  field: string;
  value: any;
  context: Record<string, any>;
  disabled?: boolean;
  onChange: (v: string) => void;
}

interface CascadingSelectState {
  options: AutoOption[];
  loading: boolean;
}

export default class CascadingSelect extends Component<CascadingSelectProps, CascadingSelectState> {
  constructor(props: CascadingSelectProps) {
    super(props);
    this.state = { options: [], loading: false };
  }

  componentDidMount() {
    this.fetchOptions();
  }

  componentDidUpdate(prev: CascadingSelectProps) {
    if (this.depsSig(prev.context) !== this.depsSig(this.props.context)) this.fetchOptions();
  }

  private depsSig(context: Record<string, any>): string {
    const { [this.props.field]: _own, ...rest } = context || {};
    return JSON.stringify(rest);
  }

  private fetchOptions() {
    const { owner, field, context } = this.props;
    if (!owner || !field) return;
    this.setState({ loading: true });
    axios
      .post(`/api/modules/${owner}/autocomplete/${field}`, { input: "", values: context || {} })
      .then((res: any) => {
        const opts: AutoOption[] = (res.data && res.data.options) || [];
        this.setState({ options: opts, loading: false });
      })
      .catch(() => this.setState({ options: [], loading: false }));
  }

  render() {
    const { value, disabled, onChange } = this.props;
    return (
      <select
        className="form-control"
        value={value ?? ""}
        disabled={disabled || this.state.loading}
        onChange={(e) => onChange(e.target.value)}
      >
        <option value="">—</option>
        {this.state.options.map((o) => (
          <option key={o.value} value={o.value}>
            {o.label}
          </option>
        ))}
      </select>
    );
  }
}
