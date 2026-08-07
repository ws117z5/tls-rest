import React, { Component, ChangeEvent } from "react";

interface JsonEditProps {
    id?: string;
    label?: string;
    value?: object | string;
    disabled?: boolean;
    required?: boolean;
    placeholder?: string;
    height?: number;
    onChange?: (value: object) => void;
    className?: string;
}

interface JsonEditState {
    jsonString: string;
    isValid: boolean;
    error: string;
}

class JsonEdit extends Component<JsonEditProps, JsonEditState> {
    static defaultProps = {
        value: {},
        disabled: false,
        required: false,
        height: 150,
        className: "",
    };

    constructor(props: JsonEditProps) {
        super(props);
        
        const initialValue = this.props.value || {};
        const jsonString = typeof initialValue === 'string' 
            ? initialValue 
            : JSON.stringify(initialValue, null, 2);

        this.state = {
            jsonString,
            isValid: true,
            error: ""
        };
    }

    componentDidUpdate(prevProps: JsonEditProps) {
        if (prevProps.value !== this.props.value) {
            const value = this.props.value || {};
            const jsonString = typeof value === 'string' 
                ? value 
                : JSON.stringify(value, null, 2);
            this.setState({ 
                jsonString,
                isValid: true,
                error: ""
            });
        }
    }

    handleChange = (e: ChangeEvent<HTMLTextAreaElement>) => {
        const jsonString = e.target.value;
        this.setState({ jsonString });

        try {
            const parsed = JSON.parse(jsonString);
            this.setState({ 
                isValid: true, 
                error: "" 
            });
            
            if (this.props.onChange) {
                this.props.onChange(parsed);
            }
        } catch (error) {
            this.setState({ 
                isValid: false, 
                error: error instanceof Error ? error.message : "Invalid JSON"
            });
        }
    };

    render() {
        const { 
            id, 
            label, 
            disabled, 
            required, 
            placeholder,
            height,
            className 
        } = this.props;
        const { jsonString, isValid, error } = this.state;

        return (
            <div className={`form-group ${className}`}>
                {label && (
                    <label htmlFor={id} className="form-label">
                        {label}
                        {required && <span className="text-danger"> *</span>}
                    </label>
                )}
                <textarea
                    id={id}
                    className={`form-control ${!isValid ? 'is-invalid' : ''}`}
                    value={jsonString}
                    disabled={disabled}
                    required={required}
                    placeholder={placeholder || "Enter JSON..."}
                    style={{ height: `${height}px`, fontFamily: 'monospace' }}
                    onChange={this.handleChange}
                />
                {!isValid && (
                    <div className="invalid-feedback">
                        {error}
                    </div>
                )}
                {isValid && jsonString && (
                    <small className="form-text text-muted">
                        Valid JSON ✓
                    </small>
                )}
            </div>
        );
    }
}

export default JsonEdit;