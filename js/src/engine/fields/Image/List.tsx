import React from "react";
import { imageUrl, normalizeRefs } from "@controllers/images";

// Compact list-cell rendering: the first thumbnail plus a "+N" badge.
interface ImageListProps {
    value?: any;
}

const ImageList: React.FC<ImageListProps> = ({ value }) => {
    const refs = normalizeRefs(value);
    if (refs.length === 0) return <span className="text-muted">-</span>;

    return (
        <div className="d-flex align-items-center" style={{ gap: 4 }}>
            <img
                src={imageUrl(refs[0].id)}
                alt={refs[0].filename || String(refs[0].id)}
                style={{ width: 40, height: 40, objectFit: "cover", border: "1px solid #ddd" }}
            />
            {refs.length > 1 && <span className="badge bg-secondary">+{refs.length - 1}</span>}
        </div>
    );
};

export default ImageList;