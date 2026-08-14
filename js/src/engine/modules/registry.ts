import React from "react";

// Webpack 5's import.meta.webpackContext (used below for build-time discovery)
// is not in the default TypeScript lib and we don't depend on
// @types/webpack-env, so declare just the surface we use. Kept here via
// `declare global` so it travels with this file regardless of tsconfig include.
declare global {
    interface ImportMeta {
        webpackContext(
            request: string,
            options?: {
                recursive?: boolean;
                regExp?: RegExp;
                mode?: "sync" | "eager" | "weak" | "lazy" | "lazy-once";
            }
        ): {
            keys(): string[];
            <T = any>(id: string): T;
            resolve(id: string): string;
            id: string | number;
        };
    }
}

// Per-module custom presentation, discovered automatically.
//
// A module needs NO manual registration to work — the generic ModulePage renders
// any module in any mode from its fieldset. To override how a module looks in a
// given mode, drop a file in that module's directory:
//
//   modules/<module>/list.tsx      -> replaces the standard list
//   modules/<module>/view.tsx      -> replaces the standard view
//   modules/<module>/edit.tsx      -> replaces the standard edit form
//   modules/<module>/create.tsx    -> replaces the standard create form
//   modules/<module>/filters.tsx   -> replaces the standard list filter bar
//
// The file's default export is picked up at build time (via
// import.meta.webpackContext) and used automatically; there is nothing to wire
// up here. Filenames are matched
// case-insensitively, so List.tsx and list.tsx both work. Any mode without an
// override falls back to the generic presentation.
//
// list/view/edit/create receive ModuleViewProps (a full-page presentation).
// filters receives ModuleFiltersProps (just the filter bar above the list).

export interface ModuleViewProps {
    module: string;                 // module name, e.g. "users"
    mode: string;                   // "list" | "view" | "edit" | "create"
    data: any[];                    // rows (list mode)
    record: any;                    // single record (view/edit/create)
    modes: string[];                // modes the current user may perform
    navigate: (to: string) => void; // client-side navigation
    reload: () => void;             // re-fetch current data
    submit: (form: any) => void;    // persist (edit/create)
    remove: (row: any) => void;     // delete a row
}

// One declared list filter, as described by the backend (GET /<module> returns a
// "Filters" array built from the module's filters.go).
export interface ModuleFilterMeta {
    name: string;                   // query parameter name
    type: string;                   // field type, drives the input widget
    label: string;                  // display label
    match?: string;                 // "", "contains", "prefix", "suffix"
    options?: Record<string, any>;  // extra field options
}

export interface ModuleFiltersProps {
    module: string;                             // module name
    meta: ModuleFilterMeta[];                   // declared filters from the backend
    values: Record<string, any>;                // current (draft) filter values
    onChange: (name: string, value: any) => void; // update one draft value
    onApply: () => void;                        // apply drafts -> reload list
    onReset: () => void;                        // clear all filters -> reload list
}

export type ModuleViews = Partial<{
    list: React.ComponentType<ModuleViewProps>;
    view: React.ComponentType<ModuleViewProps>;
    edit: React.ComponentType<ModuleViewProps>;
    create: React.ComponentType<ModuleViewProps>;
    filters: React.ComponentType<ModuleFiltersProps>;
}>;

// The override filenames we look for in each module directory. This must stay in
// sync with the literal passed to import.meta.webpackContext below (webpack can
// only statically analyse a literal RegExp there, not a variable).
const OVERRIDE_KEY = /^\.\/([^/]+)\/(list|view|edit|create|filters)\.tsx$/i;

// module name -> discovered overrides. Built at build time from the filesystem.
const registry: Record<string, ModuleViews> = {};

// import.meta.webpackContext scans this directory (engine/modules) one level
// deep for the override files above. It is resolved by webpack at build time —
// matched files are bundled, unmatched module directories cost nothing.
const context = import.meta.webpackContext(".", {
    recursive: true,
    regExp: /^\.\/[^/]+\/(list|view|edit|create|filters)\.tsx$/i,
});

context.keys().forEach((key: string) => {
    const match = OVERRIDE_KEY.exec(key);
    if (!match) return;

    const moduleName = match[1];
    const mode = match[2].toLowerCase() as keyof ModuleViews;

    const mod = context(key);
    const component = (mod && (mod.default || mod)) as ModuleViews[keyof ModuleViews];
    if (!component) return;

    if (!registry[moduleName]) registry[moduleName] = {};
    (registry[moduleName] as any)[mode] = component;
});

// getModuleViews returns the overrides for a module, tolerating case differences
// between the backend module name and its directory name.
export function getModuleViews(module: string): ModuleViews {
    return registry[module] || registry[module.toLowerCase()] || {};
}

export default registry;