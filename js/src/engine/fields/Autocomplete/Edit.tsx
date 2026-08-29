import React, { Component } from "react";
import axios from "axios";

interface AutocompleteEditProps {
  id?: string;
  fieldName?: string;
  module?: string;
  value?: string;
  placeholder?: string;
  disabled?: boolean;
  required?: boolean;
  className?: string;
  onChange?: (value: string) => void;
}

interface AutocompleteEditState {
  suggestions: string[];
  open: boolean;
  loading: boolean;
}

// AutocompleteEdit renders a text input with server-backed type-ahead. As the
// user types it POSTs {input} to /api/modules/{module}/autocomplete/{field},
// which runs the field's WithAutocomplete config (function/sql/source).
class AutocompleteEdit extends Component<AutocompleteEditProps, AutocompleteEditState> {
  private timer: any = null;
  state: AutocompleteEditState = { suggestions: [], open: false, loading: false };

  private field(): string {
    return this.props.fieldName || this.props.id || "";
  }

  private fetchSuggestions(input: string) {
    const owner = this.props.module;
    const field = this.field();
    if (!owner || !field) return;
    this.setState({ loading: true });
    axios
      .post(`/api/modules/${owner}/autocomplete/${field}`, { input })
      .then((res: any) => {
        const opts: string[] = (res.data && res.data.options) || [];
        this.setState({ suggestions: opts, open: opts.length > 0, loading: false });
      })
      .catch(() => this.setState({ suggestions: [], open: false, loading: false }));
  }

  private onInput = (e: React.ChangeEvent<HTMLInputElement>) => {
    const v = e.target.value;
    if (this.props.onChange) this.props.onChange(v);
    if (this.timer) clearTimeout(this.timer);
    this.timer = setTimeout(() => this.fetchSuggestions(v), 200);
  };

  private pick(s: string) {
    if (this.props.onChange) this.props.onChange(s);
    this.setState({ open: false, suggestions: [] });
  }

  render() {
    const { value, placeholder, disabled, required, className } = this.props;
    return (
      <div className="autocomplete-field" style={{ position: "relative" }}>
        <input
          type="text"
          className={`form-control ${className || ""}`}
          value={value || ""}
          placeholder={placeholder}
          disabled={disabled}
          required={required}
          onChange={this.onInput}
          onFocus={() => value && this.fetchSuggestions(value)}
          onBlur={() => setTimeout(() => this.setState({ open: false }), 150)}
          autoComplete="off"
        />
        {this.state.open && (
          <ul
            className="list-group"
            style={{ position: "absolute", zIndex: 1000, width: "100%", maxHeight: 220, overflowY: "auto" }}
          >
            {this.state.suggestions.map((s, i) => (
              <li
                key={i}
                className="list-group-item list-group-item-action"
                style={{ cursor: "pointer", padding: "4px 10px" }}
                onMouseDown={() => this.pick(s)}
              >
                {s}
              </li>
            ))}
          </ul>
        )}
      </div>
    );
  }
}

export default AutocompleteEdit;