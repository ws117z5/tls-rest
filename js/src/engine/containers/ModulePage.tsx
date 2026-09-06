import React, { useCallback, useEffect, useState } from "react";
import axios from "axios";

import { FieldsetProvider, FieldsetForm, FieldsetList, FieldsetFilters, MODES } from "../fields";
import {
    getModuleViews,
    ModuleViews,
    ModuleFilterMeta,
} from "@engine/controllers/registry";
import { FormLayoutBridge, WithLayout } from "@engine/fields/FormLayout";
import { Fieldset } from "@engine/pages";
import Auth from "@engine/controllers/auth";

// The generic page for any backend module. Given a module name, its API
// endpoint, and a mode, it loads the right data and renders the fieldset —
// list via FieldsetList, view/edit/create via FieldsetForm. No per-module code
// is required; a module may still override any mode by dropping a file in its
// directory (list.tsx / view.tsx / edit.tsx / create.tsx / filters.tsx), which
// is discovered automatically by the module registry.
//
// Modes are gated by `modes` (the user's permitted modes from /api/modules):
// action buttons for create/edit/delete only appear when allowed, and the
// per-mode routes themselves are only registered when permitted (see app.tsx),
// so this is defence-in-depth on top of the backend's own checks.

export type ModeName = "list" | "view" | "edit" | "create";

const MODE_MAP: Record<ModeName, number> = {
    list: MODES.LIST,
    view: MODES.VIEW,
    edit: MODES.EDIT,
    create: MODES.CREATE,
};

export interface ModulePageProps {
    module: string;
    endpoint: string; // path segment without leading slash, e.g. "posts"
    mode: ModeName;
    modes?: string[];
    title?: string;
    // Injected by AbstractComponent:
    navigate?: (to: string) => void;
    params?: { id?: string };
}

const ModulePage: React.FC<ModulePageProps> = ({
    module,
    endpoint,
    mode,
    modes = [],
    title,
    navigate,
    params,
}) => {
    const base = "/" + endpoint;
    const id = params?.id;

    const [data, setData] = useState<any[]>([]);
    const [record, setRecord] = useState<any>(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);

    // Layout override (admin-only): "default" uses the module's custom layout when
    // one exists; "system" forces the generic engine rendering. Saved per
    // module/mode. A non-admin's localStorage value is ignored (see useCustom).
    const layoutKey = `layout:${module}:${mode}`;
    const [layoutPref, setLayoutPref] = useState<string>(() => {
        try {
            return localStorage.getItem(layoutKey) || "default";
        } catch {
            return "default";
        }
    });

    // List filters. `draftFilters` is what the user is editing; `appliedFilters`
    // is what is actually sent to the server (only changes on Apply/Reset, so we
    // don't refetch on every keystroke). `filtersMeta` is the set of declared
    // filters the backend returns with the list response.
    const [filtersMeta, setFiltersMeta] = useState<ModuleFilterMeta[]>([]);
    const [draftFilters, setDraftFilters] = useState<Record<string, any>>({});
    const [appliedFilters, setAppliedFilters] = useState<Record<string, any>>({});

    // List pagination. `page` is the requested page; `pageInfo` is what the
    // server reported with the last list response (drives the pager UI).
    const [page, setPage] = useState(1);
    const [pageInfo, setPageInfo] = useState<{ page: number; limit: number; total: number } | null>(null);

    const go = useCallback(
        (to: string) => {
            if (navigate) navigate(to);
        },
        [navigate]
    );

    const can = (m: string) => modes.indexOf(m) !== -1;
    const heading = title || module.charAt(0).toUpperCase() + module.slice(1);

    const load = useCallback(async () => {
        setLoading(true);
        setError(null);
        try {
            if (mode === "list") {
                // Only send non-empty filter values, as plain query params
                // (the backend accepts ?name=value for declared filters).
                const activeParams: Record<string, any> = { page };
                Object.keys(appliedFilters).forEach((k) => {
                    const v = appliedFilters[k];
                    if (v !== "" && v !== undefined && v !== null) activeParams[k] = v;
                });
                const res = await axios.get(base, { params: activeParams });
                setData(res.data?.Data || []);
                setFiltersMeta(res.data?.Filters || []);
                // Capture server-reported pagination so FieldsetList can render the pager.
                if (res.data?.Total !== undefined) {
                    setPageInfo({
                        page: res.data?.Page ?? page,
                        limit: res.data?.Limit ?? (res.data?.Data?.length || 0),
                        total: res.data?.Total ?? 0,
                    });
                } else {
                    setPageInfo(null);
                }
            } else if ((mode === "view" || mode === "edit") && id) {
                const res = await axios.get(`${base}/${id}`);
                const d = res.data?.Data;
                // Single-record endpoints may return {Data:[record]}, {Data:record}, or the record itself.
                setRecord(Array.isArray(d) ? d[0] : d ?? res.data);
            } else {
                setRecord({}); // create
            }
        } catch (e: any) {
            setError(e?.message || "Failed to load");
        } finally {
            setLoading(false);
        }
    }, [base, mode, id, appliedFilters, page]);

    // Reset pagination and filters when navigating to a different module, so
    // state from one module (e.g. page 51 in access_log) doesn't leak into the
    // next (users would otherwise request ?page=51).
    useEffect(() => {
        setPage(1);
        setDraftFilters({});
        setAppliedFilters({});
    }, [module]);

    useEffect(() => {
        load();
    }, [load]);

    const remove = useCallback(
        async (row: any) => {
            try {
                await axios.delete(`${base}/${row.id}`);
                load();
            } catch (e) {
                console.error("Delete failed:", e);
            }
        },
        [base, load]
    );

    const bulkRemove = useCallback(
        async (rows: any[]) => {
            try {
                await Promise.all(rows.map((row) => axios.delete(`${base}/${row.id}`)));
            } catch (e) {
                console.error("Bulk delete failed:", e);
            }
            load(); // single reload after all deletes
        },
        [base, load]
    );

    const submit = useCallback(
        async (form: any) => {
            try {
                if (mode === "create") await axios.post(base, form);
                else if (mode === "edit" && id) await axios.put(`${base}/${id}`, form);
                go(base);
            } catch (e) {
                console.error("Save failed:", e);
            }
        },
        [base, mode, id, go]
    );

    // Filter handlers, handed to the filter bar (custom or default).
    const onFilterChange = useCallback((name: string, value: any) => {
        setDraftFilters((prev) => ({ ...prev, [name]: value }));
    }, []);
    const applyFilters = useCallback(() => {
        setPage(1);
        setAppliedFilters(draftFilters);
    }, [draftFilters]);
    const resetFilters = useCallback(() => {
        setPage(1);
        setDraftFilters({});
        setAppliedFilters({});
    }, []);

    // Auto-discovered per-module overrides (list/view/edit/create/filters).
    const views: ModuleViews = getModuleViews(module);

    const fmMode = MODE_MAP[mode];
    const isView = mode === "view";

    // A full-page override for the current mode takes over entirely.
    const Custom = views[mode as keyof ModuleViews] as
        | React.ComponentType<any>
        | undefined;

    const isAdmin = Auth.isAdmin();
    // Only admins may force the system layout; a non-admin who hand-edits
    // localStorage still gets the custom layout.
    const showSystem = isAdmin && layoutPref === "system";
    const useCustom = !!Custom && !showSystem;

    const toggleLayout = () => {
        const next = layoutPref === "system" ? "default" : "system";
        setLayoutPref(next);
        try {
            localStorage.setItem(layoutKey, next);
        } catch {
            // ignore storage failures
        }
    };

    // Button shown only to admins when a custom layout exists for this mode. Its
    // label is the current value ("default" | "system").
    const LayoutToggle: React.FC = () =>
        Custom && isAdmin ? (
            <button
                className="btn btn-outline-secondary"
                title="Switch between this module's custom layout and the system default"
                onClick={toggleLayout}
            >
                {layoutPref === "system" ? "system" : "default"}
            </button>
        ) : null;

    if (useCustom && mode !== "list") {
    // The parent container owns the standard chrome — module name, current mode,
    // and the standard actions (Save in edit/create, Back always). The custom
    // component only lays out the fields; it receives the form values and field
    // options via WithLayout so it can inspect data/options.
    const isEditable = mode === "edit" || mode === "create";
    return (
        <div className="container-fluid pt-4">
            <div className="d-flex justify-content-between align-items-center mb-3">
                <h1 className="h4 mb-0">
                    {heading} — {mode}
                </h1>
                <div className="d-flex gap-2">
                    {isEditable && (
                        <button className="btn btn-primary" onClick={() => submit(record)}>
                            Save
                        </button>
                    )}
                    {isView && can("edit") && (
                        <button className="btn btn-primary" onClick={() => go(`${base}/${id}/edit`)}>
                            Edit
                        </button>
                    )}
                    <LayoutToggle />
                    <button className="btn btn-secondary" onClick={() => go(base)}>
                        Back
                    </button>
                </div>
            </div>
            <div className="card">
                <div className="card-body">
                <FieldsetProvider module={module} mode={fmMode}>
                    <FormLayoutBridge
                        formData={record}
                        setRecord={setRecord}
                        mode={fmMode}
                        module={module}
                    >
                        <WithLayout
                            component={Custom}
                            extra={{
                                module,
                                mode,
                                data,
                                record,
                                modes,
                                navigate: go,
                                reload: load,
                                submit,
                                remove,
                            }}
                        />
                    </FormLayoutBridge>
                </FieldsetProvider>
                </div>
            </div>
        </div>
    );
    }

    if (loading) {
        return (
            <div className="d-flex justify-content-center p-4">
                <div className="spinner-border" role="status">
                    <span className="sr-only">Loading...</span>
                </div>
            </div>
        );
    }

    if (error) {
        return <div className="alert alert-danger m-3">{error}</div>;
    }

    if (mode === "list") {
        // Custom filter bar if the module provides one, else the standard bar
        // built from the backend's declared filters.
        const FiltersComp = views.filters || FieldsetFilters;
        // A custom list layout (e.g. an image grid) replaces the standard table,
        // but keeps the Create button, filter bar and layout toggle around it.
        const CustomList = useCustom ? (Custom as React.ComponentType<any>) : null;
        return (
            <div className="container-fluid pt-4">
                <div className="d-flex justify-content-between align-items-center mb-3">
                    <h1 className="h4 mb-0">{heading}</h1>
                    <div className="d-flex gap-2">
                        {can("create") && (
                            <button className="btn btn-primary" onClick={() => go(`${base}/create`)}>
                                Create
                            </button>
                        )}
                        <LayoutToggle />
                    </div>
                </div>
                <FiltersComp
                    module={module}
                    meta={filtersMeta}
                    values={draftFilters}
                    onChange={onFilterChange}
                    onApply={applyFilters}
                    onReset={resetFilters}
                />
                {CustomList ? (
                    <CustomList
                        module={module}
                        mode="list"
                        data={data}
                        record={null}
                        modes={modes}
                        navigate={go}
                        reload={load}
                        submit={submit}
                        remove={remove}
                    />
                ) : (
                    <FieldsetProvider module={module} mode={MODES.LIST}>
                        <FieldsetList
                            data={data}
                            sortable
                            showActions
                            onView={can("view") ? (row: any) => go(`${base}/${row.id}`) : undefined}
                            onRowDoubleClick={can("view") ? (row: any) => go(`${base}/${row.id}`) : undefined}
                            onEdit={can("edit") ? (row: any) => go(`${base}/${row.id}/edit`) : undefined}
                            onDelete={can("delete") ? remove : undefined}
                            onBulkDelete={can("delete") ? bulkRemove : undefined}
                            pagination={
                                pageInfo
                                    ? { ...pageInfo, onPageChange: (p: number) => setPage(p) }
                                    : undefined
                            }
                        />
                    </FieldsetProvider>
                )}
            </div>
        );
    }

    return (
        <div className="container-fluid pt-4">
            <div className="d-flex justify-content-between align-items-center mb-3">
                <h1 className="h4 mb-0">
                    {heading} — {mode}
                </h1>
                <div className="d-flex gap-2">
                    {isView && can("edit") && (
                        <button className="btn btn-primary" onClick={() => go(`${base}/${id}/edit`)}>
                            Edit
                        </button>
                    )}
                    <LayoutToggle />
                    <button className="btn btn-secondary" onClick={() => go(base)}>
                        Back
                    </button>
                </div>
            </div>
            <div className="card">
                <div className="card-body">
                    <FieldsetProvider module={module} mode={fmMode}>
                        <FieldsetForm
                            mode={fmMode}
                            data={record || {}}
                            onSubmit={isView ? undefined : submit}
                            disabled={isView}
                        />
                    </FieldsetProvider>
                </div>
            </div>
        </div>
    );
};

export default ModulePage;