// Token-based date/time formatter used by the Date field components. Supported
// tokens: YYYY, YY, MM, M, DD, D, HH, H, mm, m, ss, s. Anything else (separators,
// spaces) passes through. Longer tokens are matched first.
export function formatDate(value: string | number | Date | null | undefined, format: string): string {
    if (value === null || value === undefined || value === "") return "";
    const d = value instanceof Date ? value : new Date(value);
    if (isNaN(d.getTime())) return String(value);

    const pad = (n: number, len = 2) => String(n).padStart(len, "0");
    const map: Record<string, string> = {
        YYYY: String(d.getFullYear()),
        YY: pad(d.getFullYear() % 100),
        MM: pad(d.getMonth() + 1),
        M: String(d.getMonth() + 1),
        DD: pad(d.getDate()),
        D: String(d.getDate()),
        HH: pad(d.getHours()),
        H: String(d.getHours()),
        mm: pad(d.getMinutes()),
        m: String(d.getMinutes()),
        ss: pad(d.getSeconds()),
        s: String(d.getSeconds()),
    };
    return (format || "YYYY-MM-DD").replace(/YYYY|YY|MM|M|DD|D|HH|H|mm|m|ss|s/g, (t) => map[t] ?? t);
}