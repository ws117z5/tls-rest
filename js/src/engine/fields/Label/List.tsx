import React from "react";
import { parseLabels } from "./shared";

interface LabelListProps {
    value?: any;
}

const MAX_SHOWN = 3;

// Compact list-cell rendering: the first few labels + a "+N" badge, matching Image/List's convention.
const LabelList: React.FC<LabelListProps> = ({ value }) => {
    const labels = parseLabels(value);
    if (labels.length === 0) return <span className="text-muted">—</span>;
    const shown = labels.slice(0, MAX_SHOWN);
    return (
        <span className="d-inline-flex flex-wrap align-items-center" style={{ gap: 4 }}>
            {shown.map((l) => (
                <span key={l} className="badge bg-secondary-subtle text-body">
                    {l}
                </span>
            ))}
            {labels.length > shown.length && <span className="badge bg-secondary">+{labels.length - shown.length}</span>}
        </span>
    );
};

export default LabelList;
