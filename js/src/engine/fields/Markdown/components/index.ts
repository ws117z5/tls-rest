import type React from "react";

// Whitelisted components that markdown content is allowed to render.
//
// This is the safe answer to "can content use components?": NOT arbitrary JSX
// evaluation (which would be remote code execution on stored content), but a
// fixed allow-list. Drop a component in this folder and it becomes usable from
// markdown via a directive named after the file:
//
//   components/note.tsx      ->  :::note{type=warning} … :::   (block)
//                                :note[inline text]           (inline)
//
// The file's default export is discovered at build time (import.meta.web-
// packContext, same mechanism as the module override registry) and mapped by its
// lowercased filename. A directive whose name is not in this folder is not a
// component — it degrades to plain text, so content can never invoke anything
// that hasn't been vetted and placed here. Directive attributes arrive as string
// props; each component decides how to use them.

// Local type for webpack 5's import.meta.webpackContext (plain cast — no global
// augmentation, so this stays ordinary module code).
type WebpackContext = {
    keys(): string[];
    <T = any>(id: string): T;
    resolve(id: string): string;
};
type WebpackImportMeta = {
    webpackContext(
        request: string,
        options?: { recursive?: boolean; regExp?: RegExp }
    ): WebpackContext;
};

const context = (import.meta as unknown as WebpackImportMeta).webpackContext(".", {
    recursive: false,
    regExp: /^\.\/[^/]+\.tsx$/,
});

const allowedComponents: Record<string, React.ComponentType<any>> = {};

context.keys().forEach((key: string) => {
    const match = /^\.\/([^/]+)\.tsx$/.exec(key);
    if (!match) return;

    const name = match[1].toLowerCase();
    const mod = context(key);
    const component = mod && (mod.default || mod);
    if (component) allowedComponents[name] = component;
});

// The set of directive names content may use, derived from the folder.
export const allowedNames: Set<string> = new Set(Object.keys(allowedComponents));

export default allowedComponents;