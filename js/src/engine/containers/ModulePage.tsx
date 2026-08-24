import React, { useCallback, useEffect, useState } from "react";
import axios from "axios";

import { FieldsetProvider, FieldsetForm, FieldsetList, FieldsetFilters, MODES } from "../fields";
import {
    getModuleViews,
    ModuleViews,
    ModuleFilterMeta,
} from "@engine/controllers/registry";

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

    // A full-page override for the current mode takes over entirely.
    const Custom = views[mode as keyof ModuleViews] as
        | React.ComponentType<any>
        | undefined;
    if (Custom) {
        return (
            <Custom
                module={module}
                mode={mode}
                data={data}
                record={record}
                modes={modes}
                navigate={go}
                reload={load}
                submit={submit}
                remove={remove}
            />
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
        return (
            <div className="container-fluid pt-4">
                <div className="d-flex justify-content-between align-items-center mb-3">
                    <h1 className="h4 mb-0">{heading}</h1>
                    {can("create") && (
                        <button className="btn btn-primary" onClick={() => go(`${base}/create`)}>
                            Create
                        </button>
                    )}
                </div>
                <FiltersComp
                    module={module}
                    meta={filtersMeta}
                    values={draftFilters}
                    onChange={onFilterChange}
                    onApply={applyFilters}
                    onReset={resetFilters}
                />
                <FieldsetProvider module={module} mode={MODES.LIST}>
                    <FieldsetList
                        data={data}
                        sortable
                        showActions
                        onView={can("view") ? (row: any) => go(`${base}/${row.id}`) : undefined}
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
            </div>
        );
    }

    const fmMode = MODE_MAP[mode];
    const isView = mode === "view";

    return (
        <div className="container-fluid pt-4">
            <div className="d-flex justify-content-between align-items-center mb-3">
                <h1 className="h4 mb-0">
                    {heading} — {mode}
                </h1>
                <button className="btn btn-secondary" onClick={() => go(base)}>
                    Back
                </button>
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