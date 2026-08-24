import React, { useState } from "react";
import { imageUrl, normalizeRefs, ImageRef } from "@engine/modules/images/controllers/images";

// Thumb renders one image thumbnail that expands to a full-size lightbox on
// click. Shared by the Image list-cell and view renderers.
interface ThumbProps {
  refItem: ImageRef;
  size: number;
  fit: "cover" | "contain";
}

const overlayStyle: React.CSSProperties = {
  position: "fixed",
  inset: 0,
  background: "rgba(0,0,0,0.8)",
  display: "flex",
  alignItems: "center",
  justifyContent: "center",
  zIndex: 3000,
  cursor: "zoom-out",
  padding: 24,
};

const fullStyle: React.CSSProperties = {
  maxWidth: "95vw",
  maxHeight: "95vh",
  objectFit: "contain",
  boxShadow: "0 4px 24px rgba(0,0,0,0.5)",
};

const Thumb: React.FC<ThumbProps> = ({ refItem, size, fit }) => {
  const [open, setOpen] = useState(false);
  const alt = refItem.filename || String(refItem.id ?? "");

  return (
    <>
      <img
        src={imageUrl(refItem)}
        alt={alt}
        onClick={() => setOpen(true)}
        style={{
          width: size,
          height: size,
          objectFit: fit,
          border: "1px solid #ddd",
          cursor: "zoom-in",
        }}
      />
      {open && (
        <div style={overlayStyle} role="dialog" aria-modal="true" onClick={() => setOpen(false)}>
          <img src={imageUrl(refItem)} alt={alt} style={fullStyle} />
        </div>
      )}
    </>
  );
};

// helper so callers can go straight from a raw field value to refs.
export { normalizeRefs };
export default Thumb;