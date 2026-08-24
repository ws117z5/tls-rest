import React, { useCallback, useEffect, useRef, useState, ChangeEvent } from "react";
import { ImageRef, imageUrl, normalizeRefs, processImage } from "@engine/modules/images/controllers/images";

// Editable image field: click upload, the backend processes and stores each
// image, and the returned reference(s) are held in the field value and shown as
// thumbnails. Single by default; set the field option `multiple` for many.
interface ImageEditProps {
    id?: string;        // field name (used as the "field" on upload)
    module?: string;    // owning module (from the fieldset context)
    value?: any;
    onChange?: (value: ImageRef[]) => void;
    disabled?: boolean;
    multiple?: boolean; // from field.options
}

const ImageEdit: React.FC<ImageEditProps> = ({ id, module, value, onChange, disabled, multiple }) => {
    const [refs, setRefs] = useState<ImageRef[]>(normalizeRefs(value));
    const [uploading, setUploading] = useState(false);
    const inputRef = useRef<HTMLInputElement>(null);

    // Re-sync when the form resets/replaces the value (e.g. loading a record).
    useEffect(() => {
        setRefs(normalizeRefs(value));
    }, [value]);

    const emit = useCallback(
        (next: ImageRef[]) => {
            setRefs(next);
            onChange?.(next);
        },
        [onChange]
    );

    const handleFiles = async (e: ChangeEvent<HTMLInputElement>) => {
        const files = e.target.files;
        if (!files || files.length === 0) return;
        setUploading(true);
        try {
            const uploaded: ImageRef[] = [];
            for (let i = 0; i < files.length; i++) {
                uploaded.push(await processImage(files[i], module || "", id || ""));
            }
            const next = multiple ? [...refs, ...uploaded] : uploaded.slice(-1);
            emit(next);
        } catch (err) {
            console.error("Image upload failed:", err);
        } finally {
            setUploading(false);
            if (inputRef.current) inputRef.current.value = "";
        }
    };

    const remove = (idx: number) => emit(refs.filter((_, i) => i !== idx));

    return (
        <div className="image-edit">
            <label className="btn btn-sm btn-outline-primary mb-2">
                {uploading ? "Uploading..." : multiple ? "Upload images" : "Upload image"}
                <input
                    ref={inputRef}
                    type="file"
                    accept="image/*"
                    multiple={!!multiple}
                    hidden
                    disabled={disabled || uploading}
                    onChange={handleFiles}
                />
            </label>

            {refs.length > 0 && (
                <div className="d-flex flex-wrap" style={{ gap: 8 }}>
                    {refs.map((img, idx) => (
                        <div key={img.id ?? idx} className="text-center" style={{ width: 90 }}>
                            <img
                                src={imageUrl(img)}
                                alt={img.filename || String(img.id)}
                                style={{
                                    width: 80,
                                    height: 80,
                                    objectFit: "cover",
                                    border: "1px solid #ddd",
                                }}
                            />
                            {!disabled && (
                                <div>
                                    <button
                                        type="button"
                                        className="btn btn-sm btn-link text-danger p-0"
                                        onClick={() => remove(idx)}
                                    >
                                        remove
                                    </button>
                                </div>
                            )}
                        </div>
                    ))}
                </div>
            )}
        </div>
    );
};

export default ImageEdit;