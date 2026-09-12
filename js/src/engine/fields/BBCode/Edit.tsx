import React, { Component, ChangeEvent } from "react";
import { BB_TOOLS, wrapSelection, renderBBCode } from "@engine/fields/BBCode/controllers/bbcode";
import { t as translate, subscribe } from "@engine/i18n";

// A pure BBCode text editor: toolbar tags + preview. Image handling is NOT done
// here — images belong to a dedicated Image field on the module (see the Image
// field type). This keeps the editor focused on text and avoids the old
// post-specific image coupling.

interface BBCodeEditProps {
    id?: string;
    label?: string;
    value?: string;
    width?: string | number;
    height?: string | number;
    onChange?: (value: string) => void;
    disabled?: boolean;
}

interface BBCodeEditState {
    value: string;
    showPreview: boolean;
}

class BBCodeEdit extends Component<BBCodeEditProps, BBCodeEditState> {
    private textarea = React.createRef<HTMLTextAreaElement>();

    constructor(props: BBCodeEditProps) {
        super(props);
        this.state = {
            value: props.value || "",
            showPreview: false,
        };
    }

    private unsubscribeI18n?: () => void;
    componentDidMount() {
        this.unsubscribeI18n = subscribe(() => this.forceUpdate());
    }
    componentWillUnmount() {
        this.unsubscribeI18n?.();
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

    render() {
        const { width = "600px", height = "300px", disabled } = this.props;
        const { value, showPreview } = this.state;

        return (
            <div className="bbcode-edit">
                <div className="bbcode-toolbar btn-group mb-1" role="group">
                    {BB_TOOLS.map((t) => (
                        <button
                            key={t.label}
                            type="button"
                            className="btn btn-sm btn-outline-secondary"
                            title={translate(t.title)}
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
                        {showPreview ? translate("Edit") : translate("Preview")}
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
                        placeholder={translate("Write using BBCode: [b]bold[/b], [i]italic[/i], ...")}
                    />
                )}
            </div>
        );
    }
}

export default BBCodeEdit;