import React from "react";
import PropTypes from "prop-types";
import { BaseFieldProps } from "@engine/fields/Field";

// Enhanced Input component that works with fieldset system
interface EnhancedInputProps extends BaseFieldProps {
  field: any;
  value: string;
  type?: string;
  onChange?: (value:string) => {};
  disabled: boolean;
  className: string;
  placeholder?: string;
  autoComplete?: string;
  maxLength?: number;
}

const EnhancedInput: React.FC<EnhancedInputProps> = ({ 
  field,
  value = "",
  onChange,
  disabled = false,
  className = "",
  type = "text",
  placeholder,
  autoComplete,
  maxLength
}) => {
  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (onChange) {
      onChange(e.target.value);
    }
  };

  const finalPlaceholder = placeholder || field.description || field.label;
  const isRequired = field.required;
  const isDisabled = disabled || field.readonly;

  return (
    <div className={`form-group ${className}`}>
      {field.label && (
        <label htmlFor={field.name} className="form-label">
          {field.label}
          {isRequired && <span className="text-danger"> *</span>}
        </label>
      )}
      
      <input
        id={field.name}
        name={field.name}
        type={type}
        className="form-control"
        value={value}
        onChange={handleChange}
        disabled={isDisabled}
        required={isRequired}
        placeholder={finalPlaceholder}
        autoComplete={autoComplete}
        maxLength={maxLength || field.validation?.maxLength}
      />
      
      {field.description && (
        <small className="form-text text-muted">
          {field.description}
        </small>
      )}
    </div>
  );
};

// Legacy Input component for backward compatibility
interface LegacyInputProps {
  label: string;
  text: string;
  type: string;
  id: string;
  value: string;
  handleChange: (e: React.ChangeEvent<HTMLInputElement>) => void;
}

const LegacyInput: React.FC<LegacyInputProps> = ({ 
  label, 
  text, 
  type, 
  id, 
  value, 
  handleChange 
}) => (
  <div className="form-group">
    <label htmlFor={label}>{text}</label>
    <input
      type={type}
      className="form-control"
      id={id}
      value={value}
      onChange={handleChange}
      required
    />
  </div>
);

LegacyInput.propTypes = {
  label: PropTypes.string.isRequired,
  text: PropTypes.string.isRequired,
  type: PropTypes.string.isRequired,
  id: PropTypes.string.isRequired,
  value: PropTypes.string.isRequired,
  handleChange: PropTypes.func.isRequired
};

// Export both versions for compatibility
export { EnhancedInput };
export default LegacyInput;