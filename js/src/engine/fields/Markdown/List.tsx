import React from "react";
import MarkdownRender from "./MarkdownRender";

interface MarkdownViewProps {
    value?: string;
    className?: string;
}

// Renders a markdown string. Rendering goes through react-markdown, which does
// not emit raw HTML from the source, so this is safe against markup injection.
const MarkdownView: React.FC<MarkdownViewProps> = ({ value, className }) => (
    <MarkdownRender value={value || ""} className={`markdown-view ${className || ""}`} />
);

export default MarkdownView;