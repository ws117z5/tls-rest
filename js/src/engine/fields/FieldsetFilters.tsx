import React from "react";
import { FIELD_TYPES } from "./FieldsetProvider";
import type { ModuleFilterMeta, ModuleFiltersProps } from "@modules/registry";

// FieldsetFilters is the standard list filter bar. It renders one input per
// filter declared by the backend (the "Filters" array returned by GET /<module>,
// built from the module's filters.go) and reports changes upward. ModulePage
// owns the values and the reload; this component is purely presentational.
//
// A module can replace this entirely by dropping a filters.tsx in its directory
// (see registry.ts) — that component receives the same ModuleFiltersProps, so it
// is a drop-in replacement.

// Pick an <input type> (or a select) for a given filter type.
function renderInput(
    meta: ModuleFilterMeta,
    value: any,
    onChange: (value: any) => void,
    onApply: () => void
) {
    const common = {
        className: "form-control form-control-sm",
        value: value ?? "",
        onKeyDown: (e: React.KeyboardEvent) => {
            if (e.key === "Enter") onApply();
        },
    };

    switch (meta.type) {
        case FIELD_TYPES.CHECKBOX:
        case FIELD_TYPES.YES_NO:
        case FIELD_TYPES.ACTIVE_INACTIVE:
            return (
                <select
                    className="form-select form-select-sm"
                    value={value ?? ""}
                    onChange={(e) => onChange(e.target.value)}
                >
                    <option value="">Any</option>
                    <option value="true">Yes</option>
                    <option value="false">No</option>
                </select>
            );

        case FIELD_TYPES.INT:
        case FIELD_TYPES.FLOAT:
        case FIELD_TYPES.MONEY:
        case FIELD_TYPES.MONEY_WITH_CURRENCY:
            return (
                <input
                    type="number"
                    {...common}
                    onChange={(e) => onChange(e.target.value)}
                />
            );

        case FIELD_TYPES.DATE:
            return (
                <input
                    type="date"
                    {...common}
                    onChange={(e) => onChange(e.target.value)}
                />
            );

        case FIELD_TYPES.DATE_TIME:
            return (
                <input
                    type="datetime-local"
                    {...common}
                    onChange={(e) => onChange(e.target.value)}
                />
            );

        default:
            return (
                <input
                    type="text"
                    placeholder={meta.label}
                    {...common}
                    onChange={(e) => onChange(e.target.value)}
                />
            );
    }
}

const FieldsetFilters: React.FC<ModuleFiltersProps> = ({
    meta,
    values,
    onChange,
    onApply,
    onReset,
}) => {
    if (!meta || meta.length === 0) return null;

    const hasValues = Object.keys(values || {}).some(
        (k) => values[k] !== "" && values[k] !== undefined && values[k] !== null
    );

    return (
        <div className="fieldset-filters card card-body mb-3 py-2">
            <div className="row g-2 align-items-end">
                {meta.map((f) => (
                    <div className="col-auto" key={f.name}>
                        <label className="form-label mb-1 small text-muted">
                            {f.label || f.name}
                        </label>
                        {renderInput(
                            f,
                            values ? values[f.name] : "",
                            (value) => onChange(f.name, value),
                            onApply
                        )}
                    </div>
                ))}
                <div className="col-auto">
                    <button type="button" className="btn btn-primary btn-sm" onClick={onApply}>
                        Apply
                    </button>
                    {hasValues && (
                        <button
                            type="button"
                            className="btn btn-link btn-sm"
                            onClick={onReset}
                        >
                            Reset
                        </button>
                    )}
                </div>
            </div>
        </div>
    );
};

export default FieldsetFilters;