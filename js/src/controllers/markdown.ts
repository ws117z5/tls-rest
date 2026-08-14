// Markdown support for rich-text fields (replaces the old BBCode controller).
//
// Rendering is handled by react-markdown (see fields/Markdown/MarkdownRender),
// which is safe by default: it does NOT render embedded raw HTML, so stored user
// content can never inject markup. This module only holds the pure helpers the
// editor and renderer need — no React, no DOM.
//
// Images uploaded to the backend can be referenced by their id in normal image
// syntax, e.g. ![caption](<id>); resolveImageSrc turns a bare id into the
// /api/posts/images/<id> URL. Absolute http(s) and root-relative URLs are also
// allowed; anything else is dropped.

const IMG_BASE = "/api/posts/images/";

// Allow only safe link targets: http(s), root-relative, mailto, and in-page
// anchors. Everything else (javascript:, data:, etc.) is rejected.
export function safeUrl(url: string): string {
    const u = (url || "").trim();
    return /^(https?:\/\/|\/|mailto:|#)/i.test(u) ? u : "";
}

// Resolve an image src: pass through safe absolute/relative URLs, turn a bare
// backend image id into its API URL, and drop anything else.
export function resolveImageSrc(src: string): string {
    const s = (src || "").trim();
    if (!s) return "";
    if (/^(https?:\/\/|\/)/i.test(s)) return s;
    if (/^[a-zA-Z0-9_-]+$/.test(s)) return IMG_BASE + s;
    return "";
}

export interface MdTool {
    label: string;
    title: string;
    open: string;
    close: string;
}

// Editor toolbar. Each tool wraps the current selection with open/close; the
// line-prefix tools (heading/quote/list) use an empty close.
export const MD_TOOLS: MdTool[] = [
    { label: "B", title: "Bold", open: "**", close: "**" },
    { label: "I", title: "Italic", open: "_", close: "_" },
    { label: "S", title: "Strikethrough", open: "~~", close: "~~" },
    { label: "Code", title: "Inline code", open: "`", close: "`" },
    { label: "Link", title: "Link", open: "[", close: "](https://)" },
    { label: "H1", title: "Heading", open: "# ", close: "" },
    { label: "Quote", title: "Quote", open: "> ", close: "" },
    { label: "List", title: "List item", open: "- ", close: "" },
    { label: "Code block", title: "Code block", open: "\n```\n", close: "\n```\n" },
];

export interface WrapResult {
    text: string;
    selectionStart: number;
    selectionEnd: number;
}

// Wrap the [start,end) selection of `text` with open/close markers. Returns the
// new text and the selection to restore (the wrapped content, or the caret
// placed between the markers when nothing was selected).
export function wrapSelection(
    text: string,
    start: number,
    end: number,
    open: string,
    close: string
): WrapResult {
    const before = text.slice(0, start);
    const selected = text.slice(start, end);
    const after = text.slice(end);
    const next = before + open + selected + close + after;

    const caretStart = start + open.length;
    const caretEnd = selected ? caretStart + selected.length : caretStart;

    return { text: next, selectionStart: caretStart, selectionEnd: caretEnd };
}

// Insert an image at the caret using markdown syntax. `id` may be a backend image
// id (resolved to its API URL at render time) or a full URL.
export function insertImageTag(text: string, caret: number, id: string | number): WrapResult {
    const snippet = `![image](${id})`;
    const before = text.slice(0, caret);
    const after = text.slice(caret);
    return {
        text: before + snippet + after,
        selectionStart: caret + snippet.length,
        selectionEnd: caret + snippet.length,
    };
}

// Reduce markdown to a short plain-text preview for list cells (no rendering).
export function stripMarkdown(input: string, max = 140): string {
    if (!input) return "";
    let s = input;
    s = s.replace(/```[\s\S]*?```/g, " ");         // fenced code blocks
    s = s.replace(/`([^`]+)`/g, "$1");             // inline code
    s = s.replace(/!\[[^\]]*\]\([^)]*\)/g, " ");   // images
    s = s.replace(/\[([^\]]*)\]\([^)]*\)/g, "$1"); // links -> link text
    s = s.replace(/^#{1,6}\s+/gm, "");             // headings
    s = s.replace(/^\s{0,3}>\s?/gm, "");           // blockquotes
    s = s.replace(/^\s*[-*+]\s+/gm, "");           // list bullets
    s = s.replace(/[*_~]/g, "");                   // emphasis markers
    s = s.replace(/\r?\n+/g, " ").trim();          // collapse whitespace
    return s.length > max ? s.slice(0, max - 1).trimEnd() + "…" : s;
}