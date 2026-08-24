import React from "react";
import type { ModuleFiltersProps } from "@engine/controllers/registry";

// Example module override: a bespoke filter bar for the Posts list.
//
// Because this file is named filters.tsx and lives in the posts module
// directory, the registry discovers it automatically and ModulePage renders it
// in place of the standard FieldsetFilters bar — no registration needed. It
// receives ModuleFiltersProps, so it is a drop-in replacement.
//
// The filter names used here (title / public / created_from / created_to) are
// the ones the posts module declares in its backend filters.go.

const PostsFilters: React.FC<ModuleFiltersProps> = ({
    values,
    onChange,
    onApply,
    onReset,
}) => {
    const val = (name: string) => (values ? values[name] ?? "" : "");
    const onEnter = (e: React.KeyboardEvent) => {
        if (e.key === "Enter") onApply();
    };

    return (
        <div className="posts-filters card card-body mb-3 py-2">
            <div className="row g-2 align-items-end">
                <div className="col-sm-4">
                    <label className="form-label mb-1 small text-muted">Search title</label>
                    <input
                        type="text"
                        className="form-control form-control-sm"
                        placeholder="Title contains…"
                        value={val("title")}
                        onChange={(e) => onChange("title", e.target.value)}
                        onKeyDown={onEnter}
                    />
                </div>

                <div className="col-auto">
                    <label className="form-label mb-1 small text-muted">Visibility</label>
                    <select
                        className="form-select form-select-sm"
                        value={val("public")}
                        onChange={(e) => onChange("public", e.target.value)}
                    >
                        <option value="">All</option>
                        <option value="true">Public</option>
                        <option value="false">Private</option>
                    </select>
                </div>

                <div className="col-auto">
                    <label className="form-label mb-1 small text-muted">From</label>
                    <input
                        type="date"
                        className="form-control form-control-sm"
                        value={val("created_from")}
                        onChange={(e) => onChange("created_from", e.target.value)}
                    />
                </div>

                <div className="col-auto">
                    <label className="form-label mb-1 small text-muted">To</label>
                    <input
                        type="date"
                        className="form-control form-control-sm"
                        value={val("created_to")}
                        onChange={(e) => onChange("created_to", e.target.value)}
                    />
                </div>

                <div className="col-auto">
                    <button type="button" className="btn btn-primary btn-sm" onClick={onApply}>
                        Apply
                    </button>
                    <button type="button" className="btn btn-link btn-sm" onClick={onReset}>
                        Reset
                    </button>
                </div>
            </div>
        </div>
    );
};

export default PostsFilters;