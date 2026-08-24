import React, { Component } from "react";

// One selectable bit: value is the integer bit (1,2,4,…), name is the label.
interface Bit {
  value: number;
  name?: string;
  label?: string;
}

interface BitmaskEditProps {
  id?: string;
  label?: string;
  value?: number;            // the stored integer bitmask
  bits?: Bit[];              // available bits (from field.options.bits)
  options?: Bit[];           // fallback key some fields use
  disabled?: boolean;
  required?: boolean;
  onChange?: (value: number) => void;
}

// BitmaskEdit renders a checkbox per bit and keeps an integer bitmask value:
// checking a box OR-s its bit in, unchecking AND-s it out. Used for the rights
// modules' "modes" field (list|view|create|edit|delete) via widget:"bitmask".
class BitmaskEdit extends Component<BitmaskEditProps> {
  private bits(): Bit[] {
    return this.props.bits || this.props.options || [];
  }

  private toggle(bit: number, checked: boolean) {
    const current = this.props.value || 0;
    const next = checked ? current | bit : current & ~bit;
    if (this.props.onChange) this.props.onChange(next);
  }

  render() {
    const { label, required, disabled, value } = this.props;
    const mask = value || 0;
    const bits = this.bits();

    return (
      <div className="field-bitmask">
        {label && (
          <label>
            {label}
            {required && <span className="text-danger"> *</span>}
          </label>
        )}
        <div className="bitmask-options">
          {bits.map((b, idx) => {
            const on = (mask & b.value) === b.value && b.value !== 0;
            return (
              <div className="form-check" key={idx}>
                <input
                  type="checkbox"
                  className="form-check-input"
                  id={`${this.props.id || "bitmask"}-${b.value}`}
                  checked={on}
                  disabled={disabled}
                  onChange={(e) => this.toggle(b.value, e.target.checked)}
                />
                <label
                  className="form-check-label"
                  htmlFor={`${this.props.id || "bitmask"}-${b.value}`}
                >
                  {b.name || b.label || String(b.value)}
                </label>
              </div>
            );
          })}
        </div>
      </div>
    );
  }
}

export default BitmaskEdit;