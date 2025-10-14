import React, { Component, ChangeEvent } from "react";

interface Option {
    name: string;
    value?: string | number;
}

interface SelectEditProps {
    options?: Option[];
    params?: any[];
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
        const { options } = this.props;
        const { value } = this.state;

        return (
            <select value={value} onChange={this.handleChange}>
                {options &&
                    options.map((option, idx) => (
                        <option
                            key={option.value ?? idx}
                            value={option.value ?? idx}
                        >
                            {option.name}
                        </option>
                    ))}
            </select>
        );
    }
}