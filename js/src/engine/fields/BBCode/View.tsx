import React, { Component } from "react";
import { renderBBCode } from "@engine/fields/BBCode/controllers/bbcode";

interface BBCodeViewProps {
    value?: string;
    className?: string;
}

// Renders a BBCode string as HTML. renderBBCode escapes input before expanding
// the known tag set, so this is safe against markup injection.
class BBCodeView extends Component<BBCodeViewProps> {
    render() {
        const { value, className } = this.props;
        return (
            <div
                className={`bbcode-view ${className || ""}`}
                dangerouslySetInnerHTML={{ __html: renderBBCode(value || "") }}
            />
        );
    }
}

export default BBCodeView;