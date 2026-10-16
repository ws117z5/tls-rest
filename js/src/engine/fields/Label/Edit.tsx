import React, { Component } from "react";
import axios from "axios";
import { t as translate, subscribe } from "@engine/controllers/i18n";
import { parseLabels } from "./shared";

interface LabelEditProps {
    id?: string;
    fieldName?: string;
    module?: string;
    value?: any;
    placeholder?: string;
    disabled?: boolean;
    className?: string;
    onChange?: (value: string[]) => void;
    formValues?: Record<string, any>;
}

interface AutoOption {
    value: string;
    label: string;
}

interface LabelEditState {
    labels: string[];
    input: string;
    suggestions: AutoOption[];
    open: boolean;
    highlighted: number;
}

// A stable, readable color per label text — same label always gets the same chip color.
const PALETTE = ["#b58900", "#268bd2", "#2aa198", "#6c71c4", "#cb4b16", "#586e75", "#859900", "#d33682"];
function colorFor(label: string): string {
    let hash = 0;
    for (let i = 0; i < label.length; i++) hash = (hash * 31 + label.charCodeAt(i)) >>> 0;
    return PALETTE[hash % PALETTE.length];
}

// TYPE_LABEL editor: type text and press Enter (or pick a suggestion) to add a chip; click a chip's × to remove it.
// Suggestions come from the field's WithAutocomplete config, same endpoint TYPE_STRING/TYPE_INT use.
class LabelEdit extends Component<LabelEditProps, LabelEditState> {
    private timer: any = null;
    private inputRef = React.createRef<HTMLInputElement>();

    constructor(props: LabelEditProps) {
        super(props);
        this.state = { labels: parseLabels(props.value), input: "", suggestions: [], open: false, highlighted: -1 };
    }

    private unsubscribeI18n?: () => void;
    componentDidMount() {
        this.unsubscribeI18n = subscribe(() => this.forceUpdate());
    }
    componentWillUnmount() {
        this.unsubscribeI18n?.();
        if (this.timer) clearTimeout(this.timer);
    }

    componentDidUpdate(prev: LabelEditProps) {
        if (prev.value !== this.props.value) {
            const next = parseLabels(this.props.value);
            if (JSON.stringify(next) !== JSON.stringify(this.state.labels)) {
                this.setState({ labels: next });
            }
        }
    }

    private field(): string {
        return this.props.fieldName || this.props.id || "";
    }

    private emit(labels: string[]) {
        this.setState({ labels });
        this.props.onChange?.(labels);
    }

    private addLabel(raw: string) {
        const label = raw.trim();
        if (!label || this.state.labels.includes(label)) {
            this.setState({ input: "", suggestions: [], open: false, highlighted: -1 });
            return;
        }
        this.emit([...this.state.labels, label]);
        this.setState({ input: "", suggestions: [], open: false, highlighted: -1 });
    }

    private removeLabel(label: string) {
        this.emit(this.state.labels.filter((l) => l !== label));
    }

    private fetchSuggestions(input: string) {
        const owner = this.props.module;
        const field = this.field();
        if (!owner || !field || !input) {
            this.setState({ suggestions: [], open: false });
            return;
        }
        axios
            .post(`/api/modules/${owner}/autocomplete/${field}`, { input, values: this.props.formValues || {} })
            .then((res: any) => {
                const opts: AutoOption[] = ((res.data && res.data.options) || []).filter(
                    (o: AutoOption) => !this.state.labels.includes(o.value)
                );
                this.setState({ suggestions: opts, open: opts.length > 0, highlighted: -1 });
            })
            .catch(() => this.setState({ suggestions: [], open: false }));
    }

    private onInput = (e: React.ChangeEvent<HTMLInputElement>) => {
        const v = e.target.value;
        this.setState({ input: v });
        if (this.timer) clearTimeout(this.timer);
        this.timer = setTimeout(() => this.fetchSuggestions(v), 200);
    };

    private onKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
        const { suggestions, highlighted, input } = this.state;
        if (e.key === "Enter") {
            e.preventDefault();
            if (highlighted >= 0 && suggestions[highlighted]) {
                this.addLabel(suggestions[highlighted].value);
            } else {
                this.addLabel(input);
            }
        } else if (e.key === "Backspace" && !input && this.state.labels.length > 0) {
            this.removeLabel(this.state.labels[this.state.labels.length - 1]);
        } else if (e.key === "ArrowDown" && suggestions.length > 0) {
            e.preventDefault();
            this.setState({ highlighted: (highlighted + 1) % suggestions.length });
        } else if (e.key === "ArrowUp" && suggestions.length > 0) {
            e.preventDefault();
            this.setState({ highlighted: (highlighted - 1 + suggestions.length) % suggestions.length });
        } else if (e.key === "Escape") {
            this.setState({ open: false, highlighted: -1 });
        }
    };

    render() {
        const { disabled, placeholder, className } = this.props;
        const { labels, input, suggestions, open, highlighted } = this.state;
        return (
            <div className={`position-relative ${className || ""}`}>
                <div
                    className="form-control d-flex flex-wrap align-items-center"
                    style={{ gap: 6, minHeight: 38, cursor: disabled ? "default" : "text" }}
                    onClick={() => this.inputRef.current?.focus()}
                >
                    {labels.map((label) => (
                        <span
                            key={label}
                            className="badge d-inline-flex align-items-center"
                            style={{ backgroundColor: colorFor(label), color: "#fff", gap: 6, fontWeight: 400 }}
                        >
                            {label}
                            {!disabled && (
                                <button
                                    type="button"
                                    className="btn-close btn-close-white"
                                    style={{ fontSize: 8 }}
                                    aria-label={translate("Remove")}
                                    onClick={(e) => {
                                        e.stopPropagation();
                                        this.removeLabel(label);
                                    }}
                                />
                            )}
                        </span>
                    ))}
                    {!disabled && (
                        <input
                            ref={this.inputRef}
                            type="text"
                            style={{
                                border: "none",
                                outline: "none",
                                background: "transparent",
                                flex: "1 1 auto",
                                minWidth: 120,
                                width: "100%",
                            }}
                            value={input}
                            placeholder={placeholder || translate("Type to add a label...")}
                            onChange={this.onInput}
                            onKeyDown={this.onKeyDown}
                            onBlur={() => setTimeout(() => this.setState({ open: false }), 150)}
                        />
                    )}
                </div>
                {open && suggestions.length > 0 && (
                    <div
                        className="dropdown-menu d-block position-absolute w-100"
                        style={{ maxHeight: 200, overflowY: "auto", zIndex: 20 }}
                    >
                        {suggestions.map((opt, i) => (
                            <button
                                key={opt.value}
                                type="button"
                                className={`dropdown-item${i === highlighted ? " active" : ""}`}
                                onMouseDown={(e) => e.preventDefault()}
                                onClick={() => this.addLabel(opt.value)}
                            >
                                {opt.label}
                            </button>
                        ))}
                    </div>
                )}
            </div>
        );
    }
}

export default LabelEdit;
