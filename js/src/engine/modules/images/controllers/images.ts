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
export function imageUrl(ref: ImageRef, opts?: { preview?: boolean }): string {
    let base: string;
    if (ref.url) {
        base = `${Config.serverURL}${ref.url.replace(/^\//, "")}`;
    } else {
        const guid = ref.uuid || String(ref.id);
        const ext = extOf(ref);
        const name = ext ? `${guid}.${ext}` : guid;
        base = `${Config.serverURL}image/${name}`;
    }
    // Preview mode asks the server for the compressed 80x80 content-aware thumb.
    return opts?.preview ? `${base}${base.includes("?") ? "&" : "?"}preview` : base;
}

/**
 * Normalize a stored image-field value into ImageRef[]. Accepts an array, a
 * JSON string, a single ref object, or null/undefined.
 */
export function normalizeRefs(value: any): ImageRef[] {
    if (!value) return [];
    let v = value;
    if (typeof v === "string") {
        const s = v.trim();
        if (!s) return [];
        try {
            v = JSON.parse(s);
        } catch {
            // Not JSON — treat as a bare uuid/guid reference (e.g. the images
            // module's "preview" field, which is just the uuid column).
            return [{ id: s, uuid: s }];
        }
    }
    if (Array.isArray(v)) return v.filter(Boolean);
    if (typeof v === "object") return [v];
    return [];
}

/**
 * iPhones upload HEIC/HEIF, which browsers can't render (and the Go stdlib can't
 * decode server-side), so the stored image would show blank. Convert those to
 * JPEG in the browser before upload; every other format is passed through
 * untouched. heic2any is loaded lazily so it only ships when actually needed.
 */
async function ensureBrowserImage(file: File): Promise<File> {
    const name = (file.name || "").toLowerCase();
    const looksHeic =
        file.type === "image/heic" ||
        file.type === "image/heif" ||
        name.endsWith(".heic") ||
        name.endsWith(".heif");
    if (!looksHeic) return file; // fast path — no decoder needed for normal images

    // heic-to bundles a current libheif (WASM), which decodes modern iPhone HEIC
    // (incl. 10-bit/HDR) that the older heic2any build rejects with ERR_LIBHEIF.
    const { heicTo } = await import("heic-to");
    let blob: Blob;
    try {
        blob = await heicTo({ blob: file, type: "image/jpeg", quality: 0.9 });
    } catch (e: any) {
        const detail =
            e && typeof e === "object" && (e.message || e.code)
                ? `${e.code ?? ""} ${e.message ?? ""}`.trim()
                : typeof e === "object"
                ? JSON.stringify(e)
                : String(e);
        // eslint-disable-next-line no-console
        console.error("heic-to error:", e);
        throw new Error(
            "This HEIC couldn't be converted (" +
                detail +
                "). On iPhone, set Settings → Camera → Formats → Most Compatible, " +
                "or upload a JPEG/PNG."
        );
    }
    if (!blob || blob.size === 0) {
        throw new Error("HEIC conversion produced an empty image");
    }
    const jpgName = file.name.replace(/\.(heic|heif)$/i, "") + ".jpg";
    return new File([blob], jpgName, { type: "image/jpeg" });
}

/**
 * Read image metadata (EXIF: dimensions, orientation, camera, capture time, GPS)
 * from the ORIGINAL file — before any HEIC->JPEG conversion, which strips EXIF.
 * Best-effort: always returns at least the basic file facts. exifr is loaded
 * lazily so it only ships when an image is uploaded.
 */
async function extractMetadata(file: File): Promise<Record<string, any>> {
    const meta: Record<string, any> = {
        original_filename: file.name || null,
        original_mime: file.type || null,
        size: typeof file.size === "number" ? file.size : null,
        last_modified: file.lastModified ? new Date(file.lastModified).toISOString() : null,
    };
    try {
        const exifr = (await import("exifr")).default as any;
        const data = await exifr.parse(file, { gps: true, tiff: true, exif: true, ifd0: true }).catch(() => null);
        if (data) {
            meta.width = data.ImageWidth ?? data.ExifImageWidth ?? null;
            meta.height = data.ImageHeight ?? data.ExifImageHeight ?? null;
            meta.make = data.Make ?? null;
            meta.model = data.Model ?? null;
            meta.orientation = data.Orientation ?? null;
            meta.taken_at =
                data.DateTimeOriginal instanceof Date
                    ? data.DateTimeOriginal.toISOString()
                    : data.DateTimeOriginal ?? null;
            if (typeof data.latitude === "number" && typeof data.longitude === "number") {
                meta.gps = { lat: data.latitude, lng: data.longitude };
            }
        }
    } catch {
        // metadata is best-effort; ignore lookup failures
    }
    // Tidy: drop null/undefined entries.
    return Object.fromEntries(Object.entries(meta).filter(([, v]) => v !== null && v !== undefined));
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
    // Capture metadata from the original file first, then convert if needed.
    const metadata = await extractMetadata(file);
    const uploadFile = await ensureBrowserImage(file);

    const form = new FormData();
    form.append("image", uploadFile);
    form.append("module", module);
    form.append("field", field);
    form.append("metadata", JSON.stringify(metadata));
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