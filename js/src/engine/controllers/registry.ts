import React from "react";

// Per-module custom presentation, auto-discovered from modules/<module>/<mode>[.<Name>].tsx — no manual registration.

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

// A mode's discovered views, keyed by name — "default" for the bare
// <mode>.tsx, or the <Name> segment of <mode>.<Name>.tsx.
export type NamedViews = Record<string, React.ComponentType<ModuleViewProps>>;

export type ModuleViews = Partial<{
    list: NamedViews;
    view: NamedViews;
    edit: NamedViews;
    create: NamedViews;
    filters: React.ComponentType<ModuleFiltersProps>;
}>;

type ViewMode = "list" | "view" | "edit" | "create";

// Must match the literal regex passed to webpackContext below (needs a static literal, not a variable).
const OVERRIDE_KEY = /^\.\/([^/]+)\/(list|view|edit|create|filters)(?:\.([A-Za-z0-9_-]+))?\.tsx$/i;

// module name -> discovered overrides. Built at build time from the filesystem.
const registry: Record<string, ModuleViews> = {};

// import.meta.webpackContext scans this directory (engine/modules) one level
// deep for the override files above. It is resolved by webpack at build time —
// matched files are bundled, unmatched module directories cost nothing.
// mode "lazy": every override becomes its own chunk, fetched when its module/mode is first shown (rendered under a Suspense boundary).
const engineContext = import.meta.webpackContext("../modules", {
    recursive: true,
    mode: "lazy",
    regExp: /^\.\/[^/]+\/(list|view|edit|create|filters)(?:\.([A-Za-z0-9_-]+))?\.tsx$/i,
});

const userContext = import.meta.webpackContext("../../modules", {
    recursive: true,
    mode: "lazy",
    regExp: /^\.\/[^/]+\/(list|view|edit|create|filters)(?:\.([A-Za-z0-9_-]+))?\.tsx$/i,
});

function loadRegistry(ctx: __WebpackModuleApi.RequireContext) {
    ctx.keys().forEach((key: string) => {
        const match = OVERRIDE_KEY.exec(key);
        if (!match) return;

        const moduleName = match[1];
        const mode = match[2].toLowerCase() as ViewMode | "filters";
        const viewName = (match[3] || "default").toLowerCase();

        const component = React.lazy(() => ctx(key));

        if (!registry[moduleName]) registry[moduleName] = {};
        const views = registry[moduleName];

        if (mode === "filters") {
            views.filters = component as React.ComponentType<ModuleFiltersProps>;
            return;
        }

        if (!views[mode]) views[mode] = {};
        (views[mode] as NamedViews)[viewName] = component as React.ComponentType<ModuleViewProps>;

        // Fallback: an edit view (of any name) also serves as that same-named
        // create view when no create.<Name>.tsx exists for it.
        if (mode === "edit" && !views.create?.[viewName]) {
            if (!views.create) views.create = {};
            views.create[viewName] = component as React.ComponentType<ModuleViewProps>;
        }
    });
}

loadRegistry(engineContext);
loadRegistry(userContext);

// getModuleViews returns the overrides for a module, tolerating case differences
// between the backend module name and its directory name.
export function getModuleViews(module: string): ModuleViews {
    return registry[module] || registry[module.toLowerCase()] || {};
}

export default registry;