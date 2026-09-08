import React, { Component, ChangeEvent } from "react";
interface Props {
  id?: string;
  value?: string;
  placeholder?: string;
  disabled?: boolean;
  required?: boolean;
  className?: string;
  width?: string | number;
  onChange?: (value: string) => void;
}
class PasswordEdit extends Component<Props> {
  render() {
    const { id, value, placeholder, disabled, required, className, onChange } = this.props;
    const width = this.props.width ?? "auto";
    return (
      <input
        id={id}
        type="password"
        autoComplete="new-password"
        className={`form-control ${className || ""}`.trim()}
        value={value ?? ""}
        placeholder={placeholder}
        disabled={disabled}
        required={required}
        style={{ width }}
        onChange={(e: ChangeEvent<HTMLInputElement>) => onChange?.(e.target.value)}
      />
    );
  }
}
export default PasswordEdit;