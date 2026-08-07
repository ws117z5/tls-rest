import React, { Component, ChangeEvent } from "react";
import axios from "axios";

interface TextEditProps {
    id?: string;
    label?: string;
    placeholder?: string;
    width?: string | number;
    height?: string | number;
    fontFamily?: string;
    fontSize?: string | number;
    value?: string;
    inputProps?: React.InputHTMLAttributes<HTMLInputElement> & React.TextareaHTMLAttributes<HTMLTextAreaElement>;
    type?: "text" | "varchar";
    autocompleteUrl?: string | null;
    onChange?: (value: string, e?: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => void;
}

interface TextEditState {
    value: string;
    suggestions: string[];
    showSuggestions: boolean;
}

class TextEdit extends Component<TextEditProps, TextEditState> {
    static defaultProps = {
        width: "auto",
        height: "auto",
        fontFamily: "inherit",
        fontSize: "1rem",
        value: "",
        inputProps: {},
        type: "text",
        autocompleteUrl: null,
    };

    constructor(props: TextEditProps) {
        super(props);
        this.state = {
            value: props.value || "",
            suggestions: [],
            showSuggestions: false,
        };
    }

    componentDidUpdate(prevProps: TextEditProps) {
        if (prevProps.value !== this.props.value) {
            this.setState({ value: this.props.value || "" });
        }
    }

    handleChange = (e: ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
        const newValue = e.target.value;
        this.setState({ value: newValue });

        if (typeof this.props.onChange === "function") {
            this.props.onChange(newValue, e);
        }
        if (this.props.autocompleteUrl) {
            this.fetchSuggestions(newValue);
        }
    };

    fetchSuggestions = async (query: string) => {
        if (!query) {
            this.setState({ suggestions: [], showSuggestions: false });
            return;
        }
        try {
            const res = await axios.get(this.props.autocompleteUrl as string, { params: { q: query } });
            this.setState({ suggestions: res.data || [], showSuggestions: true });
        } catch {
            this.setState({ suggestions: [], showSuggestions: false });
        }
    };

    handleSuggestionClick = (suggestion: string) => {
        this.setState({ value: suggestion, showSuggestions: false });
        if (this.props.onChange) {
            this.props.onChange(suggestion);
        }
    };

    render() {
        const {
            id,
            label,
            placeholder,
            width,
            height,
            fontFamily,
            fontSize,
            inputProps,
            type,
        } = this.props;

        const { value, suggestions, showSuggestions } = this.state;

        const commonProps = {
            value: value ?? this.state.value,
            onChange: this.handleChange,
            style: {
                width,
                height,
                fontFamily,
                fontSize,
                ...(inputProps && inputProps.style ? inputProps.style : {}),
            },
            placeholder,
            id,
            ...inputProps,
        };

        return (
            <div style={{ position: "relative", width }}>
                {type === "text" ? (
                    <textarea {...commonProps as React.TextareaHTMLAttributes<HTMLTextAreaElement>} />
                ) : (
                    <input type="text" {...commonProps as React.InputHTMLAttributes<HTMLInputElement>} autoComplete="off" />
                )}
                {showSuggestions && suggestions.length > 0 && (
                    <ul
                        style={{
                            position: "absolute",
                            top: "100%",
                            left: 0,
                            right: 0,
                            background: "#fff",
                            border: "1px solid #ccc",
                            zIndex: 1000,
                            margin: 0,
                            padding: 0,
                            listStyle: "none",
                            maxHeight: 150,
                            overflowY: "auto",
                        }}
                    >
                        {suggestions.map((s, i) => (
                            <li
                                key={i}
                                style={{ padding: "4px 8px", cursor: "pointer" }}
                                onMouseDown={() => this.handleSuggestionClick(s)}
                            >
                                {s}
                            </li>
                        ))}
                    </ul>
                )}
            </div>
        );
    }
}

export default TextEdit;