import React, { Component, ChangeEvent } from "react";
import { MD_TOOLS, wrapSelection } from "@controllers/markdown";
import MarkdownRender from "./MarkdownRender";

// A markdown text editor: toolbar + textarea with a live-preview toggle. Image
// handling is NOT done here — images belong to a dedicated Image field on the
// module. Uploaded images can still be embedded by id in markdown image syntax
// (![alt](<id>)); the renderer resolves the id to its API URL.

interface MarkdownEditProps {
    id?: string;
    label?: string;
    value?: string;
    width?: string | number;
    height?: string | number;
    onChange?: (value: string) => void;
    disabled?: boolean;
}

interface MarkdownEditState {
    value: string;
    showPreview: boolean;
}

class MarkdownEdit extends Component<MarkdownEditProps, MarkdownEditState> {
    private textarea = React.createRef<HTMLTextAreaElement>();

    constructor(props: MarkdownEditProps) {
        super(props);
        this.state = {
            value: props.value || "",
            showPreview: false,
        };
    }

    componentDidUpdate(prev: MarkdownEditProps) {
        if (prev.value !== this.props.value && this.props.value !== this.state.value) {
            this.setState({ value: this.props.value || "" });
        }
    }

    private emit(value: string) {
        this.setState({ value });
        this.props.onChange?.(value);
    }

    handleChange = (e: ChangeEvent<HTMLTextAreaElement>) => {
        this.emit(e.target.value);
    };

    applyTag = (open: string, close: string) => {
        const el = this.textarea.current;
        if (!el) return;
        const { text, selectionStart, selectionEnd } = wrapSelection(
            this.state.value,
            el.selectionStart,
            el.selectionEnd,
            open,
            close
        );
        this.emit(text);
        // Restore selection after React re-renders.
        requestAnimationFrame(() => {
            el.focus();
            el.setSelectionRange(selectionStart, selectionEnd);
        });
    };

    render() {
        const { width = "600px", height = "300px", disabled } = this.props;
        const { value, showPreview } = this.state;

        return (
            <div className="markdown-edit">
                <div className="markdown-toolbar btn-group mb-1" role="group">
                    {MD_TOOLS.map((t) => (
                        <button
                            key={t.label}
                            type="button"
                            className="btn btn-sm btn-outline-secondary"
                            title={t.title}
                            disabled={disabled}
                            onClick={() => this.applyTag(t.open, t.close)}
                        >
                            {t.label}
                        </button>
                    ))}
                    <button
                        type="button"
                        className="btn btn-sm btn-outline-secondary"
                        onClick={() => this.setState((s) => ({ showPreview: !s.showPreview }))}
                    >
                        {showPreview ? "Edit" : "Preview"}
                    </button>
                </div>

                {showPreview ? (
                    <div
                        className="markdown-preview form-control"
                        style={{ width, minHeight: height, overflow: "auto" }}
                    >
                        <MarkdownRender value={value} />
                    </div>
                ) : (
                    <textarea
                        ref={this.textarea}
                        className="form-control"
                        style={{ width, height }}
                        value={value}
                        disabled={disabled}
                        onChange={this.handleChange}
                        placeholder="Write in Markdown: **bold**, _italic_, [link](url), `code`, - lists…"
                    />
                )}
            </div>
        );
    }
}

export default MarkdownEdit;