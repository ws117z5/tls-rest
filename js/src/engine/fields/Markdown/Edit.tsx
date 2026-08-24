import React, { Component, ChangeEvent } from "react";
import { MD_TOOLS, wrapSelection } from "@engine/fields/Markdown/controllers/markdown";
import { processImage } from "@engine/modules/images/controllers/images";
import MarkdownRender from "./controllers/MarkdownRender";

// A markdown text editor: toolbar + textarea with a live-preview toggle. The
// image button uploads a file and inserts image markdown (![name](guid.ext)) at
// the caret; the renderer resolves the guid to its access-controlled /image URL.

interface MarkdownEditProps {
    id?: string;
    module?: string;    // owning module (for the image upload's module/field context)
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
    uploading: boolean;
}

class MarkdownEdit extends Component<MarkdownEditProps, MarkdownEditState> {
    private textarea = React.createRef<HTMLTextAreaElement>();
    private fileInput = React.createRef<HTMLInputElement>();

    constructor(props: MarkdownEditProps) {
        super(props);
        this.state = {
            value: props.value || "",
            showPreview: false,
            uploading: false,
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

    // Insert a string at the current caret (replacing any selection) and place
    // the caret just after it.
    private insertAtCaret(snippet: string) {
        const el = this.textarea.current;
        const value = this.state.value;
        const start = el ? el.selectionStart : value.length;
        const end = el ? el.selectionEnd : value.length;
        const next = value.slice(0, start) + snippet + value.slice(end);
        this.emit(next);
        const caret = start + snippet.length;
        requestAnimationFrame(() => {
            if (!el) return;
            el.focus();
            el.setSelectionRange(caret, caret);
        });
    }

    // Image button → open the file picker.
    private pickImage = () => this.fileInput.current?.click();

    private handleImageFile = async (e: ChangeEvent<HTMLInputElement>) => {
        const file = e.target.files?.[0];
        if (file) {
            this.setState({ uploading: true });
            try {
                const ref = await processImage(file, this.props.module || "", this.props.id || "");
                const alt = (ref.filename || "image").replace(/[\[\]]/g, "");
                const target = ref.uuid ? ref.uuid : String(ref.id);
                const ext =
                    ref.filename && ref.filename.includes(".")
                        ? "." + ref.filename.split(".").pop()
                        : "";
                // Reference by guid.ext; the renderer resolves it to /image/<guid>.
                this.insertAtCaret(`![${alt}](${target}${ext})`);
            } catch (err) {
                console.error("Image upload failed:", err);
            } finally {
                this.setState({ uploading: false });
                if (this.fileInput.current) this.fileInput.current.value = "";
            }
        }
    };

    render() {
        const { width = "600px", height = "300px", disabled } = this.props;
        const { value, showPreview, uploading } = this.state;

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
                        title="Insert image at cursor"
                        disabled={disabled || uploading}
                        onClick={this.pickImage}
                    >
                        {uploading ? "…" : "🖼"}
                    </button>
                    <button
                        type="button"
                        className="btn btn-sm btn-outline-secondary"
                        onClick={() => this.setState((s) => ({ showPreview: !s.showPreview }))}
                    >
                        {showPreview ? "Edit" : "Preview"}
                    </button>
                    <input
                        ref={this.fileInput}
                        type="file"
                        accept="image/*"
                        hidden
                        onChange={this.handleImageFile}
                    />
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