import React from "react";

// Optional per-module custom views.
//
// A module needs NO entry here to work — the generic ModulePage renders any
// module in any mode from its fieldset. To override how a module looks/behaves
// in a given mode, drop a component in the module directory (e.g.
// modules/users/List.tsx or modules/users/View.tsx) and register it below. The
// generic page is used for any mode without an override.

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

export type ModuleViews = Partial<{
    list: React.ComponentType<ModuleViewProps>;
    view: React.ComponentType<ModuleViewProps>;
    edit: React.ComponentType<ModuleViewProps>;
    create: React.ComponentType<ModuleViewProps>;
}>;

// module name -> custom views. Empty by default.
const registry: Record<string, ModuleViews> = {
    // Example:
    // users: { list: UsersList, view: UsersView },
};

export default registry;