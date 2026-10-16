import axios from "axios";

// AppConfig holds the user's effective config (theme, date_format, ...) resolved
// server-side (global < group < user). Loaded once on startup via /api/config and
// read synchronously by components (with sane defaults before it loads).
class AppConfig {
    private static config: Record<string, string> = {
        theme: "light",
        date_format: "YYYY-MM-DD",
    };

    static async load(): Promise<void> {
        try {
            const res = await axios.get("/api/config", { headers: { "X-Request-Type": "api" } });
            if (res.data && typeof res.data === "object") {
                AppConfig.config = { ...AppConfig.config, ...res.data };
            }
        } catch {
            // keep defaults
        }
        AppConfig.applyTheme(AppConfig.config.theme);
    }

    // applyTheme drives Bootstrap 5.3's built-in color modes by setting
    // data-bs-theme on <html>. Also mirrored to body[data-theme] for any custom CSS.
    static applyTheme(theme: string): void {
        const t = theme === "dark" ? "dark" : "light";
        try {
            // The served CSS is Bootstrap 4 (no data-bs-theme), so drive dark mode
            // via data-theme on <html>, which css/theme-dark.css keys on. Also set
            // data-bs-theme so a future Bootstrap 5 upgrade works unchanged.
            document.documentElement.setAttribute("data-theme", t);
            document.documentElement.setAttribute("data-bs-theme", t);
        } catch {
            // no DOM (SSR / tests)
        }
    }

    // setTheme changes the theme immediately in the running app (e.g. a toggle),
    // without waiting for a reload. Persisting it is done via the config module.
    static setTheme(theme: string): void {
        AppConfig.config.theme = theme === "dark" ? "dark" : "light";
        AppConfig.applyTheme(AppConfig.config.theme);
    }

    static get(key: string): string {
        return AppConfig.config[key];
    }

    static dateFormat(): string {
        return AppConfig.config.date_format || "YYYY-MM-DD";
    }

    static theme(): string {
        return AppConfig.config.theme || "light";
    }

    // syncWithGoogleDefault: whether a newly-created event's own "sync with
    // Google Calendar" checkbox (events.sync_with_google) should start
    // checked — the user's own profile preference (Profile.tsx).
    static syncWithGoogleDefault(): boolean {
        return AppConfig.config.sync_with_google_default !== "false";
    }

    // moduleView resolves the admin-configured default view (ModulePage's layout toggle) for module+mode at this
    // user's scope (global < group < user; see engine/controllers/config.Resolve), or undefined if none is set.
    static moduleView(module: string, mode: string): string | undefined {
        const raw = AppConfig.config.module_views;
        if (!raw) return undefined;
        try {
            const map = JSON.parse(raw) as Record<string, string>;
            return map[`${module}:${mode}`];
        } catch {
            return undefined;
        }
    }
}

export default AppConfig;