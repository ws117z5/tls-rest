// Shared helpers for the Image field type.
//
// An image field's value is an array of references ({id, uuid, filename}); the
// bytes live server-side. Upload goes through the generic images controller,
// which runs the (module, field) preprocessor before storing. Display is by the
// stable /image/{guid}.{ext} URL, and serving is access-controlled on the server
// (an image inherits the access of the record it's attached to, unless it has a
// per-image override).

import axios from "axios";
import Config from "@engine/Config";

export interface ImageRef {
    id: number | string;
    uuid?: string;
    filename?: string;
    mime_type?: string;
    url?: string;
}

// Derive a file extension for the image URL from the filename or mime type.
function extOf(ref: ImageRef): string {
    if (ref.filename) {
        const i = ref.filename.lastIndexOf(".");
        if (i >= 0 && i < ref.filename.length - 1) return ref.filename.slice(i + 1).toLowerCase();
    }
    switch (ref.mime_type) {
        case "image/png": return "png";
        case "image/jpeg": return "jpg";
        case "image/gif": return "gif";
        case "image/webp": return "webp";
        case "image/svg+xml": return "svg";
    }
    return "";
}

/**
 * URL that serves the raw image bytes (used for <img src>). Prefers the guid
 * (uuid) reference /image/{guid}.{ext}; falls back to the id for legacy refs.
 * Serving is access-controlled server-side.
 */
export function imageUrl(ref: ImageRef): string {
    if (ref.url) return `${Config.serverURL}${ref.url.replace(/^\//, "")}`;
    const guid = ref.uuid || String(ref.id);
    const ext = extOf(ref);
    const name = ext ? `${guid}.${ext}` : guid;
    return `${Config.serverURL}image/${name}`;
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
 * Upload + preprocess one image for a given module/field. Optionally attach it
 * to a record (recordId) so it inherits that record's access, and/or set a
 * per-image access override. The backend runs the matching preprocessor, stores
 * the bytes, and returns the new reference (including its /image/{guid} url).
 */
export async function processImage(
    file: File,
    module: string,
    field: string,
    opts?: { recordId?: number | string; access?: number }
): Promise<ImageRef> {
    const form = new FormData();
    form.append("image", file);
    form.append("module", module);
    form.append("field", field);
    if (opts?.recordId !== undefined && opts.recordId !== null) {
        form.append("record_id", String(opts.recordId));
    }
    if (opts?.access !== undefined && opts.access !== null) {
        form.append("access", String(opts.access));
    }

    const res = await axios.post(`${Config.serverURL}api/images/process`, form, {
        headers: { "Content-Type": "multipart/form-data" },
    });

    return res.data as ImageRef;
}