import React from "react";
import { stripMarkdown } from "./controllers/markdown";

interface MarkdownListProps {
    value?: string;
    className?: string;
}

// List cells show a short plain-text preview rather than rendered markdown, to
// keep table rows compact. stripMarkdown removes syntax and truncates.
const MarkdownList: React.FC<MarkdownListProps> = ({ value, className }) => (
    <span className={`markdown-list text-muted ${className || ""}`}>
        {stripMarkdown(value || "")}
    </span>
);

export default MarkdownList;