import React from "react";
import { parseLabels } from "./shared";

interface LabelViewProps {
    value?: any;
}

const LabelView: React.FC<LabelViewProps> = ({ value }) => {
    const labels = parseLabels(value);
    if (labels.length === 0) return <span className="text-muted">—</span>;
    return (
        <span className="d-inline-flex flex-wrap gap-1">
            {labels.map((l) => (
                <span key={l} className="badge bg-secondary-subtle text-body">
                    {l}
                </span>
            ))}
        </span>
    );
};

export default LabelView;
