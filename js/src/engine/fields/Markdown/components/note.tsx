import React from "react";

// Example whitelisted markdown component.
//
// Because this file lives in the components folder and is named note.tsx, the
// registry allows content to render it via a directive:
//
//   :::note{type=warning}
//   **Heads up** — this is a callout.
//   :::
//
// or inline: :note[quick aside]
//
// Directive attributes ({type=warning}) arrive as string props. Note that props
// are always strings/plain data — content cannot pass functions or execute code,
// which is what keeps this safe.

type NoteType = "info" | "tip" | "success" | "warning" | "danger";

const CLASS_BY_TYPE: Record<NoteType, string> = {
    info: "alert-info",
    tip: "alert-primary",
    success: "alert-success",
    warning: "alert-warning",
    danger: "alert-danger",
};

interface NoteProps {
    type?: string;
    children?: React.ReactNode;
}

const Note: React.FC<NoteProps> = ({ type = "info", children }) => {
    const cls = CLASS_BY_TYPE[(type as NoteType)] || CLASS_BY_TYPE.info;
    return (
        <div className={`markdown-note alert ${cls} my-2`} role="note">
            {children}
        </div>
    );
};

export default Note;