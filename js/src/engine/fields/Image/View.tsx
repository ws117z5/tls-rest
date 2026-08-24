import React from "react";
import { normalizeRefs } from "@engine/modules/images/controllers/images";
import Thumb from "./Thumb";

// Read-only display of an image field's stored reference(s); each expands on click.
interface ImageViewProps {
    value?: any;
}

const ImageView: React.FC<ImageViewProps> = ({ value }) => {
    const refs = normalizeRefs(value);
    if (refs.length === 0) return <span>-</span>;

    return (
        <div className="d-flex flex-wrap" style={{ gap: 8 }}>
            {refs.map((img, i) => (
                <Thumb key={img.id ?? i} refItem={img} size={120} fit="contain" />
            ))}
        </div>
    );
};

export default ImageView;