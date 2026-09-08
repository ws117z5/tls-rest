import React, { Component, ChangeEvent } from "react";

interface FloatEditProps {
    id?: string;
    value?: string | number;
    placeholder?: string;
    disabled?: boolean;
    required?: boolean;
    className?: string;
    step?: string | number;
    min?: number;
    max?: number;
    onChange?: (value: string, e?: ChangeEvent<HTMLInputElement>) => void;
    width?: string | number;
    inputProps?: React.InputHTMLAttributes<HTMLInputElement>;
}

// Editable numeric input for TYPE_INT / TYPE_FLOAT / TYPE_MONEY. Controlled by the
// parent form: it reports raw string values via onChange so the fieldset keeps a
// single source of truth (and empty stays empty rather than coercing to 0).
class FloatEdit extends Component<FloatEditProps> {
    static defaultProps = {
        step: "any",
        className: "",
        width: "auto",
    };

    handleChange = (e: ChangeEvent<HTMLInputElement>) => {
        this.props.onChange?.(e.target.value, e);
    };

    render() {
        const {
            id, value, placeholder, disabled, required,
            className, step, min, max, inputProps, width,
        } = this.props;
        return (
            <input
                id={id}
                type="number"
                className={`form-control ${className || ""}`.trim()}
                value={value ?? ""}
                placeholder={placeholder}
                disabled={disabled}
                required={required}
                step={step}
                min={min}
                max={max}
                style={{ width }}
                onChange={this.handleChange}
                {...inputProps}
            />
        );
    }
}

export default FloatEdit;