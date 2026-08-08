import React, { Component, ChangeEvent } from "react";
import {
    BB_TOOLS,
    wrapSelection,
    insertImageTag,
    renderBBCode,
} from "@controllers/bbcode";

import {
    uploadPostImage,
    loadPostImages,
    imageUrl,
    PostImageMeta,
} from "@modules/posts/postImages";

interface BBCodeEditProps {
    id?: string;
    label?: string;
    value?: string;
    width?: string | number;
    height?: string | number;
    postId?: number | string;
    onChange?: (value: string) => void;
    disabled?: boolean;
}

interface BBCodeEditState {
    value: string;
    images: PostImageMeta[];
    uploading: boolean;
    showPreview: boolean;
}

class BBCodeEdit extends Component<BBCodeEditProps, BBCodeEditState> {
    private textarea = React.createRef<HTMLTextAreaElement>();

    constructor(props: BBCodeEditProps) {
        super(props);
        this.state = {
            value: props.value || "",
            images: [],
            uploading: false,
            showPreview: false,
        };
    }

    componentDidMount() {
        // Load existing images into the cache once, then display thumbnails.
        loadPostImages(this.props.postId)
            .then((images) => this.setState({ images }))
            .catch(() => {
                /* no images yet */
            });
    }

    componentDidUpdate(prev: BBCodeEditProps) {
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

    insertImage = (id: number | string) => {
        const el = this.textarea.current;
        const caret = el ? el.selectionStart : this.state.value.length;
        const { text, selectionStart } = insertImageTag(this.state.value, caret, id);
        this.emit(text);
        requestAnimationFrame(() => {
            if (el) {
                el.focus();
                el.setSelectionRange(selectionStart, selectionStart);
            }
        });
    };

    handleUpload = async (e: ChangeEvent<HTMLInputElement>) => {
        const files = e.target.files;
        if (!files || files.length === 0) return;
        this.setState({ uploading: true });
        try {
            const uploaded: PostImageMeta[] = [];
            for (let i = 0; i < files.length; i++) {
                uploaded.push(await uploadPostImage(files[i], this.props.postId));
            }
            this.setState((s) => ({ images: [...s.images, ...uploaded], uploading: false }));
        } catch (err) {
            console.error("Image upload failed:", err);
            this.setState({ uploading: false });
        } finally {
            e.target.value = "";
        }
    };

    render() {
        const { width = "600px", height = "300px", disabled } = this.props;
        const { value, images, uploading, showPreview } = this.state;

        return (
            <div className="bbcode-edit">
                <div className="bbcode-toolbar btn-group mb-1" role="group">
                    {BB_TOOLS.map((t) => (
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
                        className="bbcode-preview form-control"
                        style={{ width, minHeight: height, overflow: "auto" }}
                        dangerouslySetInnerHTML={{ __html: renderBBCode(value) }}
                    />
                ) : (
                    <textarea
                        ref={this.textarea}
                        className="form-control"
                        style={{ width, height }}
                        value={value}
                        disabled={disabled}
                        onChange={this.handleChange}
                        placeholder="Write your post using BBCode: [b]bold[/b], [img]id[/img], ..."
                    />
                )}

                <div className="bbcode-images mt-2">
                    <label className="btn btn-sm btn-outline-primary mb-1">
                        {uploading ? "Uploading..." : "Upload image"}
                        <input
                            type="file"
                            accept="image/*"
                            multiple
                            hidden
                            disabled={disabled || uploading}
                            onChange={this.handleUpload}
                        />
                    </label>

                    {images.length > 0 && (
                        <div className="d-flex flex-wrap" style={{ gap: "8px" }}>
                            {images.map((img) => (
                                <div key={img.id} className="text-center" style={{ width: 90 }}>
                                    <img
                                        src={imageUrl(img.id)}
                                        alt={img.filename || String(img.id)}
                                        style={{
                                            width: 80,
                                            height: 80,
                                            objectFit: "cover",
                                            border: "1px solid #ddd",
                                            cursor: "pointer",
                                        }}
                                        title={`Insert [img]${img.id}[/img]`}
                                        onClick={() => this.insertImage(img.id)}
                                    />
                                    <div>
                                        <code
                                            style={{ cursor: "pointer", fontSize: 11 }}
                                            title="Insert into body"
                                            onClick={() => this.insertImage(img.id)}
                                        >
                                            [img]{img.id}[/img]
                                        </code>
                                    </div>
                                </div>
                            ))}
                        </div>
                    )}
                </div>
            </div>
        );
    }
}

export default BBCodeEdit;