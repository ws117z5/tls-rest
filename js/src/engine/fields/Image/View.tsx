import React from "react";
import { imageUrl, normalizeRefs } from "@controllers/images";

// Read-only display of an image field's stored reference(s).
interface ImageViewProps {
    value?: any;
}

const ImageView: React.FC<ImageViewProps> = ({ value }) => {
    const refs = normalizeRefs(value);
    if (refs.length === 0) return <span>-</span>;

    return (
        <div className="d-flex flex-wrap" style={{ gap: 8 }}>
            {refs.map((img, i) => (
                <img
                    key={img.id ?? i}
                    src={imageUrl(img)}
                    alt={img.filename || String(img.id)}
                    style={{ maxWidth: 200, maxHeight: 200, objectFit: "contain", border: "1px solid #eee" }}
                />
            ))}
        </div>
    );
};

export default ImageView;