import axios from "axios";

export interface AutoOption {
    value: string;
    label: string;
}

// Resolves a stored id to its label via a {"id"} request (see resolveAutocompleteView server-side). Shared by Edit/View.
export async function resolveAutocompleteLabel(
    module: string | undefined,
    field: string,
    value: string | number | undefined,
    formValues: Record<string, any> | undefined
): Promise<string | null> {
    if (value === undefined || value === null || value === "" || !module || !field) return null;
    const str = String(value); // a stored id may arrive as a JS number
    try {
        const res = await axios.post(`/api/modules/${module}/autocomplete/${field}`, {
            id: str,
            values: formValues || {},
        });
        const opts: AutoOption[] = (res.data && res.data.options) || [];
        return opts.length > 0 ? opts[0].label : null;
    } catch {
        return null;
    }
}
