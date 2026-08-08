// BBCode support for post bodies.
//
// renderBBCode() converts a BBCode string into safe HTML. It escapes HTML first
// (so user input can never inject markup), then rewrites a known set of BBCode
// tags. [img]<id>[/img] resolves to an uploaded post image served by the
// backend at /posts/images/<id>.
//
// insertTag()/wrapSelection() are used by the editor toolbar to wrap the current
// textarea selection with a tag.

const IMG_BASE = "/api/posts/images/";

// Escape HTML so raw input can't inject markup.
function escapeHtml(s: string): string {
    return s
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;")
        .replace(/'/g, "&#39;");
}

// Only allow safe http(s)/relative URLs in [url]; otherwise drop the href.
function safeUrl(url: string): string {
    const u = url.trim();
    if (/^(https?:\/\/|\/)/i.test(u)) {
        return u.replace(/"/g, "%22");
    }
    return "";
}

// Image ids are backend-generated (numeric id or uuid); restrict to a safe set.
function safeImageId(id: string): string {
    return /^[a-zA-Z0-9_-]+$/.test(id.trim()) ? id.trim() : "";
}

export function renderBBCode(input: string): string {
    if (!input) return "";

    let out = escapeHtml(input);

    // [img]id[/img]  ->  <img src="/posts/images/id">
    out = out.replace(/\[img\]([^\[\]]+)\[\/img\]/gi, (_m, id: string) => {
        const safe = safeImageId(id);
        return safe
            ? `<img class="bb-img" src="${IMG_BASE}${safe}" alt="image ${safe}" loading="lazy">`
            : "";
    });

    // [url=href]text[/url] and [url]href[/url]
    out = out.replace(/\[url=([^\[\]]+)\]([\s\S]*?)\[\/url\]/gi, (_m, href: string, text: string) => {
        const safe = safeUrl(href);
        return safe ? `<a href="${safe}" rel="noopener noreferrer" target="_blank">${text}</a>` : text;
    });
    out = out.replace(/\[url\]([^\[\]]+)\[\/url\]/gi, (_m, href: string) => {
        const safe = safeUrl(href);
        return safe ? `<a href="${safe}" rel="noopener noreferrer" target="_blank">${safe}</a>` : href;
    });

    // [color=...] and [size=NN]
    out = out.replace(/\[color=([a-zA-Z0-9#]+)\]([\s\S]*?)\[\/color\]/gi,
        (_m, color: string, text: string) => `<span style="color:${color.replace(/[^a-zA-Z0-9#]/g, "")}">${text}</span>`);
    out = out.replace(/\[size=(\d{1,2})\]([\s\S]*?)\[\/size\]/gi,
        (_m, size: string, text: string) => `<span style="font-size:${Math.min(parseInt(size, 10), 48)}px">${text}</span>`);

    // Simple inline/block tags
    out = out.replace(/\[b\]([\s\S]*?)\[\/b\]/gi, "<strong>$1</strong>");
    out = out.replace(/\[i\]([\s\S]*?)\[\/i\]/gi, "<em>$1</em>");
    out = out.replace(/\[u\]([\s\S]*?)\[\/u\]/gi, "<u>$1</u>");
    out = out.replace(/\[s\]([\s\S]*?)\[\/s\]/gi, "<s>$1</s>");
    out = out.replace(/\[quote\]([\s\S]*?)\[\/quote\]/gi, "<blockquote>$1</blockquote>");
    out = out.replace(/\[code\]([\s\S]*?)\[\/code\]/gi, "<pre><code>$1</code></pre>");

    // Lists: [list] ... [*] item ... [/list]
    out = out.replace(/\[list\]([\s\S]*?)\[\/list\]/gi, (_m, body: string) => {
        const items = body
            .split(/\[\*\]/)
            .map((s) => s.trim())
            .filter(Boolean)
            .map((s) => `<li>${s}</li>`)
            .join("");
        return `<ul>${items}</ul>`;
    });

    // Newlines -> <br> (do this last so block tags above aren't broken).
    out = out.replace(/\r?\n/g, "<br>");

    return out;
}

export interface BBToolItem {
    label: string;
    title: string;
    open: string;
    close: string;
}

// Toolbar definition used by the editor.
export const BB_TOOLS: BBToolItem[] = [
    { label: "B", title: "Bold", open: "[b]", close: "[/b]" },
    { label: "I", title: "Italic", open: "[i]", close: "[/i]" },
    { label: "U", title: "Underline", open: "[u]", close: "[/u]" },
    { label: "S", title: "Strikethrough", open: "[s]", close: "[/s]" },
    { label: "URL", title: "Link", open: "[url=https://]", close: "[/url]" },
    { label: "Quote", title: "Quote", open: "[quote]", close: "[/quote]" },
    { label: "Code", title: "Code", open: "[code]", close: "[/code]" },
    { label: "List", title: "List", open: "[list][*]", close: "[/list]" },
];

export interface WrapResult {
    text: string;
    selectionStart: number;
    selectionEnd: number;
}

// Wrap the [start,end) selection of `text` with open/close tags. Returns the new
// text and the selection range placed between the tags (or around the wrapped
// content).
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

    // If nothing was selected, place the caret between the tags.
    const caretStart = selected ? start + open.length : start + open.length;
    const caretEnd = selected ? start + open.length + selected.length : caretStart;

    return { text: next, selectionStart: caretStart, selectionEnd: caretEnd };
}

// Insert an [img]id[/img] tag at the given caret position.
export function insertImageTag(text: string, caret: number, id: string | number): WrapResult {
    const tag = `[img]${id}[/img]`;
    const before = text.slice(0, caret);
    const after = text.slice(caret);
    return {
        text: before + tag + after,
        selectionStart: caret + tag.length,
        selectionEnd: caret + tag.length,
    };
}