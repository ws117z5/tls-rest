import React, { Component, ChangeEvent } from "react";

interface IntEditProps {
    id?: string;
    label?: string;
    value?: number;
    min?: number;
    max?: number;
    step?: number;
    disabled?: boolean;
    required?: boolean;
    placeholder?: string;
    onChange?: (value: number) => void;
    className?: string;
}

interface IntEditState {
    value: string;
}

class IntEdit extends Component<IntEditProps, IntEditState> {
    static defaultProps = {
        value: 0,
        step: 1,
        disabled: false,
        required: false,
        className: "",
    };

    constructor(props: IntEditProps) {
        super(props);
        this.state = {
            value: (props.value ?? "").toString(),
        };
    }

    componentDidUpdate(prevProps: IntEditProps) {
        if (prevProps.value !== this.props.value) {
            this.setState({ value: (this.props.value ?? "").toString() });
        }
    }

    handleChange = (e: ChangeEvent<HTMLInputElement>) => {
        const stringValue = e.target.value;
        this.setState({ value: stringValue });

        // Parse and validate integer
        const numValue = parseInt(stringValue, 10);
        if (!isNaN(numValue) && this.props.onChange) {
            this.props.onChange(numValue);
        } else if (stringValue === "" && this.props.onChange) {
            this.props.onChange(0);
        }
    };

    render() {
        const { 
            id, 
            label, 
            min, 
            max, 
            step, 
            disabled, 
            required, 
            placeholder,
            className 
        } = this.props;
        const { value } = this.state;

        return (
            <div className={`form-group ${className}`}>
                {label && (
                    <label htmlFor={id} className="form-label">
                        {label}
                        {required && <span className="text-danger"> *</span>}
                    </label>
                )}
                <input
                    id={id}
                    type="number"
                    className="form-control"
                    value={value}
                    min={min}
                    max={max}
                    step={step}
                    disabled={disabled}
                    required={required}
                    placeholder={placeholder || label}
                    onChange={this.handleChange}
                />
            </div>
        );
    }
}

export default IntEdit;