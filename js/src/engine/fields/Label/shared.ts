// Shared by Label/{Edit,View,List}: a TYPE_LABEL value is a JSON array of strings (or already an array, or empty/null).
export function parseLabels(value: any): string[] {
    if (Array.isArray(value)) return value.map(String);
    if (typeof value !== "string" || !value.trim()) return [];
    try {
        const v = JSON.parse(value);
        return Array.isArray(v) ? v.map(String) : [];
    } catch {
        return [];
    }
}
