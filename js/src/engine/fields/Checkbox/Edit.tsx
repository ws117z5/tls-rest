import React, { Component, ChangeEvent } from "react";

interface CheckboxEditProps {
    id?: string;
    label?: string;
    value?: boolean;
    disabled?: boolean;
    required?: boolean;
    onChange?: (value: boolean) => void;
    className?: string;
}

interface CheckboxEditState {
    checked: boolean;
}

class CheckboxEdit extends Component<CheckboxEditProps, CheckboxEditState> {
    static defaultProps = {
        value: false,
        disabled: false,
        required: false,
        className: "",
    };

    constructor(props: CheckboxEditProps) {
        super(props);
        this.state = {
            checked: props.value || false,
        };
    }

    componentDidUpdate(prevProps: CheckboxEditProps) {
        if (prevProps.value !== this.props.value) {
            this.setState({ checked: this.props.value || false });
        }
    }

    handleChange = (e: ChangeEvent<HTMLInputElement>) => {
        const checked = e.target.checked;
        this.setState({ checked });
        
        if (this.props.onChange) {
            this.props.onChange(checked);
        }
    };

    render() {
        const { id, label, disabled, required, className } = this.props;
        const { checked } = this.state;

        return (
            <div className={`form-check ${className}`}>
                <input
                    id={id}
                    type="checkbox"
                    className="form-check-input"
                    checked={checked}
                    disabled={disabled}
                    required={required}
                    onChange={this.handleChange}
                />
                {label && (
                    <label className="form-check-label" htmlFor={id}>
                        {label}
                        {required && <span className="text-danger"> *</span>}
                    </label>
                )}
            </div>
        );
    }
}

export default CheckboxEdit;