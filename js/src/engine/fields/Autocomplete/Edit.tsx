import React, { Component } from "react";
import axios from "axios";
import { AutoOption, resolveAutocompleteLabel } from "./shared";

interface AutocompleteEditProps {
  id?: string;
  fieldName?: string;
  module?: string;
  value?: string | number; // a stored id may arrive as a JS number
  placeholder?: string;
  disabled?: boolean;
  required?: boolean;
  className?: string;
  onChange?: (value: string) => void;
  formValues?: Record<string, any>; // sibling field values, e.g. config.scope_id's "scope"
}

interface AutocompleteEditState {
  suggestions: AutoOption[];
  open: boolean;
  loading: boolean;
  text: string; // shown text (label); the stored value is reported via onChange
}

// AutocompleteEdit renders a text input with server-backed type-ahead. As the
// user types it POSTs {input, values} to /api/modules/{module}/autocomplete/{field}.
// Suggestions are {value, label}: the label is shown, the value is stored.
class AutocompleteEdit extends Component<AutocompleteEditProps, AutocompleteEditState> {
  private timer: any = null;

  constructor(props: AutocompleteEditProps) {
    super(props);
    this.state = { suggestions: [], open: false, loading: false, text: String(props.value ?? "") };
  }

  componentDidMount() {
    this.resolveInitialLabel();
  }

  componentDidUpdate(prev: AutocompleteEditProps) {
    if (prev.value !== this.props.value && String(this.props.value ?? "") !== this.state.text) {
      this.setState({ text: String(this.props.value ?? "") });
      this.resolveInitialLabel();
    }
  }

  // Resolves the stored id to its label on load.
  private resolveInitialLabel() {
    const { value, module: owner, formValues } = this.props;
    const field = this.field();
    resolveAutocompleteLabel(owner, field, value, formValues).then((label) => {
      if (label) this.setState({ text: label });
    });
  }

  private field(): string {
    return this.props.fieldName || this.props.id || "";
  }

  private fetchSuggestions(input: string) {
    const owner = this.props.module;
    const field = this.field();
    if (!owner || !field) return;
    this.setState({ loading: true });
    axios
      .post(`/api/modules/${owner}/autocomplete/${field}`, {
        input,
        values: this.props.formValues || {},
      })
      .then((res: any) => {
        const opts: AutoOption[] = (res.data && res.data.options) || [];
        this.setState({ suggestions: opts, open: opts.length > 0, loading: false });
      })
      .catch(() => this.setState({ suggestions: [], open: false, loading: false }));
  }

  private onInput = (e: React.ChangeEvent<HTMLInputElement>) => {
    const v = e.target.value;
    this.setState({ text: v }); // display only — typing never reports a value, only pick() does
    if (this.timer) clearTimeout(this.timer);
    this.timer = setTimeout(() => this.fetchSuggestions(v), 200);
  };

  private pick(opt: AutoOption) {
    this.setState({ text: opt.label, open: false, suggestions: [] });
    if (this.props.onChange) this.props.onChange(opt.value);
  }

  // Left without picking: empty box clears the value, else display reverts to the stored value's label.
  private onBlur = () => {
    setTimeout(() => {
      this.setState({ open: false });
      if (!this.state.text) {
        if (this.props.value && this.props.onChange) this.props.onChange("");
        return;
      }
      resolveAutocompleteLabel(this.props.module, this.field(), this.props.value, this.props.formValues).then(
        (label) => {
          this.setState({ text: label ?? String(this.props.value ?? "") });
        }
      );
    }, 150);
  };

  render() {
    const { placeholder, disabled, required, className } = this.props;
    return (
      <div className="autocomplete-field" style={{ position: "relative", maxWidth: 280 }}>
        <input
          type="text"
          className={`form-control ${className || ""}`}
          value={this.state.text}
          placeholder={placeholder}
          disabled={disabled}
          required={required}
          onChange={this.onInput}
          onFocus={() => this.state.text && this.fetchSuggestions(this.state.text)}
          onBlur={this.onBlur}
          autoComplete="off"
        />
        {this.state.open && (
          <ul
            className="list-group"
            style={{ position: "absolute", zIndex: 1000, width: "100%", maxHeight: 220, overflowY: "auto" }}
          >
            {this.state.suggestions.map((opt, i) => (
              <li
                key={i}
                className="list-group-item list-group-item-action"
                style={{ cursor: "pointer", padding: "4px 10px" }}
                onMouseDown={() => this.pick(opt)}
              >
                {opt.label}
              </li>
            ))}
          </ul>
        )}
      </div>
    );
  }
}

export default AutocompleteEdit;
