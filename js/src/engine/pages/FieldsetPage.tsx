import React, { useCallback, useEffect, useState } from "react";
import axios from "axios";
 
import { FieldsetProvider, FieldsetForm, MODES } from "@engine/fields";
 
// A Page is a standalone, non-module screen with a single visual representation
// and NO modes (no list/create/delete, no per-mode routes). It renders one
// fieldset for one record, gated by rights: the endpoint returns the record and
// an already rights-filtered fieldset ({ Data, Fieldset:{fields:[...]} }), which
// fields are editable is decided server-side, and — when `editable` — the form
// can save back with PUT. Profile is the canonical example (the current user).
 
interface FieldsetPageProps {
    endpoint: string;      // GET returns {Data, Fieldset}; PUT saves (when editable)
    editable?: boolean;    // render an editable form (fields still gated by rights)
    moduleName?: string;   // logical fieldset name (context only; not fetched)
    title?: string;
}
 
const FieldsetPage: React.FC<FieldsetPageProps> = ({ endpoint, editable = false, moduleName = "", title }) => {
    const [data, setData] = useState<any>(null);
    const [fieldset, setFieldset] = useState<any>(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);
    const [saved, setSaved] = useState(false);
 
    const load = useCallback(async () => {
        setLoading(true);
        setError(null);
        try {
            const res = await axios.get(endpoint);
            const d = res.data?.Data;
            setData(Array.isArray(d) ? d[0] : d ?? {});
            setFieldset(res.data?.Fieldset ?? null);
        } catch (e: any) {
            setError(e?.message || "Failed to load");
        } finally {
            setLoading(false);
        }
    }, [endpoint]);
 
    useEffect(() => {
        load();
    }, [load]);
 
    const save = useCallback(
        async (form: any) => {
            try {
                await axios.put(endpoint, form);
                setSaved(true);
            } catch (e) {
                console.error("Save failed:", e);
            }
        },
        [endpoint]
    );
 
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
 
    const mode = editable ? MODES.EDIT : MODES.VIEW;
 
    return (
        <div className="container-fluid">
            {title && <h1 className="h4 mb-3">{title}</h1>}
            {saved && <div className="alert alert-success">Saved.</div>}
            <FieldsetProvider module={moduleName} mode={mode} fieldset={fieldset}>
                <FieldsetForm
                    mode={mode}
                    data={data || {}}
                    onSubmit={editable ? save : undefined}
                    disabled={!editable}
                />
            </FieldsetProvider>
        </div>
    );
};
 
export default FieldsetPage;