// Post image API helpers.
//
// Images are uploaded to the backend (stored in the database) and referenced in
// post bodies as [img]<id>[/img]. Caching is handled entirely on the server
// (see go/controllers/postimages + cache.Cache), so there is no client-side
// cache here — the browser's own HTTP cache covers repeated /api/posts/images
// requests.

import axios from "axios";
import Config from "@engine/Config";

export interface PostImageMeta {
    id: number | string;
    uuid?: string;
    filename?: string;
    mime_type?: string;
}

/** URL that serves the raw image bytes (used for <img src> thumbnails). */
export function imageUrl(id: number | string): string {
    return `${Config.serverURL}api/posts/images/${id}`;
}

/** Upload a file; the backend stores it and returns its generated id. */
export async function uploadPostImage(
    file: File,
    postId?: number | string
): Promise<PostImageMeta> {
    const form = new FormData();
    form.append("image", file);
    if (postId !== undefined && postId !== null) {
        form.append("post_id", String(postId));
    }

    const res = await axios.post(`${Config.serverURL}api/posts/images`, form, {
        headers: { "Content-Type": "multipart/form-data" },
    });

    return res.data;
}

/** Fetch image metadata for a post (optionally filtered by post id). */
export async function loadPostImages(
    postId?: number | string
): Promise<PostImageMeta[]> {
    const res = await axios.get(`${Config.serverURL}api/posts/images`, {
        params: postId != null ? { post_id: postId } : {},
    });

    return Array.isArray(res.data) ? res.data : res.data?.Data || [];
}