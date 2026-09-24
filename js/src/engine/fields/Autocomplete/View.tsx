import React, { Component } from "react";
import { resolveAutocompleteLabel } from "./shared";

interface AutocompleteViewProps {
    id?: string;
    fieldName?: string;
    module?: string;
    // A stored id may arrive as a JS number (a JSON int column decodes that way).
    value?: string | number;
    formValues?: Record<string, any>;
}

interface AutocompleteViewState {
    label: string;
}

// Read-only display: resolves the stored id to its label (see shared.ts).
class AutocompleteView extends Component<AutocompleteViewProps, AutocompleteViewState> {
    constructor(props: AutocompleteViewProps) {
        super(props);
        this.state = { label: String(props.value ?? "") };
    }

    componentDidMount() {
        this.resolveLabel();
    }

    componentDidUpdate(prev: AutocompleteViewProps) {
        if (prev.value !== this.props.value) {
            this.setState({ label: String(this.props.value ?? "") });
            this.resolveLabel();
        }
    }

    private resolveLabel() {
        const { value, module: owner, formValues } = this.props;
        const field = this.props.fieldName || this.props.id || "";
        resolveAutocompleteLabel(owner, field, value, formValues).then((label) => {
            if (label) this.setState({ label });
        });
    }

    render() {
        return <span>{this.state.label || "-"}</span>;
    }
}

export default AutocompleteView;
