import React, { Component, ChangeEvent } from "react";

interface IntEditProps {
  id?: string;
  value?: number | string;
  min?: number;
  max?: number;
  step?: number;
  disabled?: boolean;
  required?: boolean;
  placeholder?: string;
  className?: string;
  width?: string | number;
  onChange?: (value: number) => void;
}

interface IntEditState { value: string; }

// Editable integer input for TYPE_INT. Renders only the control (the field layout
// draws the label). Reports an integer via onChange; empty stays empty.
class IntEdit extends Component<IntEditProps, IntEditState> {
  static defaultProps = { step: 1, width: "auto" };

  constructor(props: IntEditProps) {
    super(props);
    this.state = { value: props.value === undefined || props.value === null ? "" : String(props.value) };
  }

  componentDidUpdate(prev: IntEditProps) {
    if (prev.value !== this.props.value) {
      const v = this.props.value;
      this.setState({ value: v === undefined || v === null ? "" : String(v) });
    }
  }

  handleChange = (e: ChangeEvent<HTMLInputElement>) => {
    const s = e.target.value;
    this.setState({ value: s });
    if (!this.props.onChange) return;
    if (s === "") { this.props.onChange(0); return; }
    const n = parseInt(s, 10);
    if (!isNaN(n)) this.props.onChange(n);
  };

  render() {
    const { id, min, max, step, disabled, required, placeholder, className, width } = this.props;
    return (
      <input
        id={id}
        type="number"
        inputMode="numeric"
        step={step ?? 1}
        className={`form-control ${className || ""}`.trim()}
        value={this.state.value}
        min={min}
        max={max}
        disabled={disabled}
        required={required}
        placeholder={placeholder}
        style={{ width }}
        onChange={this.handleChange}
      />
    );
  }
}
export default IntEdit;