import React, { Component, ChangeEvent } from "react";

interface Option {
    name: string;
    value?: string | number;
}

interface SelectEditProps {
    options?: Option[];
    params?: any | any[];
    value?: string | number;
    onChange?: (value: string | number, params?: any[]) => void;
}

interface SelectEditState {
    value: string | number;
}

export default class SelectEdit extends Component<SelectEditProps, SelectEditState> {
    static defaultProps = {
        options: [],
        params: [],
        value: "",
        onChange: () => {},
    };

    constructor(props: SelectEditProps) {
        super(props);
        this.state = {
            value: props.value ?? "",
        };
    }

    componentDidUpdate(prevProps: SelectEditProps) {
        if (prevProps.value !== this.props.value) {
            this.setState({ value: this.props.value ?? "" });
        }
    }

    handleChange = (e: ChangeEvent<HTMLSelectElement>) => {
        const newValue = e.target.value;
        this.setState({ value: newValue });
        if (this.props.onChange) {
            this.props.onChange(newValue, this.props.params);
        }
    };

    render() {
        const { options = [] } = this.props;
        const { value } = this.state;

        // Show an explicit blank option whenever the current value matches no
        // option (i.e. nothing has been picked), so the control honestly reads
        // "—" instead of silently showing the first option as if selected.
        const hasMatch = options.some(
            (o) => String(o.value ?? "") === String(value ?? "")
        );
        const opts = hasMatch ? options : [{ name: "—", value: "" }, ...options];

        return (
            <select value={value ?? ""} onChange={this.handleChange}>
                {opts.map((option, idx) => (
                    <option key={option.value ?? idx} value={option.value ?? idx}>
                        {option.name}
                    </option>
                ))}
            </select>
        );
    }
}