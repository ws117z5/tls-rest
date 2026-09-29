import React, { useState } from "react";
import { fileToDataURL } from "./imageUtil";

// A file picker + preview for a single source image, used by every panel that sends an init/start image.
const ImageDrop: React.FC<{ label: string; onChange: (dataUrl: string | null) => void; disabled?: boolean }> = ({
  label,
  onChange,
  disabled,
}) => {
  const [preview, setPreview] = useState<string | null>(null);

  const pick = async (file: File | undefined) => {
    if (!file) {
      setPreview(null);
      onChange(null);
      return;
    }
    const dataUrl = await fileToDataURL(file);
    setPreview(dataUrl);
    onChange(dataUrl);
  };

  return (
    <div>
      <div className="text-muted small text-uppercase mb-1">{label}</div>
      <input
        type="file"
        accept="image/png,image/jpeg,image/gif"
        className="form-control"
        disabled={disabled}
        onChange={(e) => pick(e.target.files?.[0])}
      />
      {preview && (
        <img src={preview} alt="" style={{ maxWidth: 160, maxHeight: 160, marginTop: 8, borderRadius: 4 }} />
      )}
    </div>
  );
};

export default ImageDrop;
