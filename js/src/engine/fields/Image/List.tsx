import React from "react";
import { normalizeRefs } from "@engine/modules/images/controllers/images";
import Thumb from "./Thumb";

// Compact list-cell rendering: the first thumbnail (click to expand) + a "+N" badge.
// listSize (field option) overrides the default 40px thumbnail, e.g. images.go's
// "preview" field sets it larger since that field IS the row's whole point there.
interface ImageListProps {
    value?: any;
    listSize?: number;
}

const ImageList: React.FC<ImageListProps> = ({ value, listSize }) => {
    const refs = normalizeRefs(value);
    if (refs.length === 0) return <span className="text-muted">-</span>;

    return (
        <div className="d-flex align-items-center" style={{ gap: 4 }}>
            <Thumb refItem={refs[0]} size={listSize || 40} fit="cover" />
            {refs.length > 1 && <span className="badge bg-secondary">+{refs.length - 1}</span>}
        </div>
    );
};

export default ImageList;