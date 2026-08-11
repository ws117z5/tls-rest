// Shared helpers for the Image field type.
//
// An image field's value is an array of references ({id, uuid, filename}); the
// bytes live server-side. Upload goes through the generic images controller,
// which runs the (module, field) preprocessor before storing. Display is by id.

import axios from "axios";
import Config from "@engine/Config";

export interface ImageRef {
    id: number | string;
    uuid?: string;
    filename?: string;
    mime_type?: string;
}

/** URL that serves the raw image bytes (used for <img src>). Public. */
export function imageUrl(id: number | string): string {
    return `${Config.serverURL}api/images/${id}`;
}

/**
 * Normalize a stored image-field value into ImageRef[]. Accepts an array, a
 * JSON string, a single ref object, or null/undefined.
 */
export function normalizeRefs(value: any): ImageRef[] {
    if (!value) return [];
    let v = value;
    if (typeof v === "string") {
        try {
            v = JSON.parse(v);
        } catch {
            return [];
        }
    }
    if (Array.isArray(v)) return v.filter(Boolean);
    if (typeof v === "object") return [v];
    return [];
}

/**
 * Upload + preprocess one image for a given module/field. The backend runs the
 * matching preprocessor, stores the bytes, and returns the new reference.
 */
export async function processImage(
    file: File,
    module: string,
    field: string
): Promise<ImageRef> {
    const form = new FormData();
    form.append("image", file);
    form.append("module", module);
    form.append("field", field);

    const res = await axios.post(`${Config.serverURL}api/images/process`, form, {
        headers: { "Content-Type": "multipart/form-data" },
    });

    return res.data as ImageRef;
}