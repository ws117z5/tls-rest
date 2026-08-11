import Auth from "@controllers/auth";

// Two kinds of navigable things:
//
//  * Backend modules — declared in go.config.json, governed by the rights
//    system, and reported (already access-filtered, with per-mode rights) by
//    GET /api/modules. Each is rendered by the generic ModulePage (or a
//    registered custom view) with per-mode routes.
//
//  * Custom pages — frontend-only React pages (graphs, tools, login, …) that
//    are not backend modules. Discovered by scanning components/pages and
//    gated by their own requiresAuth/requiresAdmin flags.

export interface BackendModuleEntry {
    name: string;
    title: string;
    href: string;       // path segment, no leading slash (e.g. "posts")
    description: string;
    modes: string[];    // modes this user may perform
}

export interface CustomPage {
    href: string;
    name: string;
    title: string;
    uuid: string;
    isPage: boolean;
    requiresAuth: boolean;
    requiresAdmin: boolean;
    component: any;
    props: Record<string, any>;
    extraRoutes: Array<{ href: string; component: any }>;
    condition?: (state: any) => boolean;
}

export default class Config {
    private static backendModules: BackendModuleEntry[] = [];
    private static customPages: CustomPage[] = [];
    private static initPromise: Promise<void> | null = null;
    public static serverURL = window.location.origin + "/";

    static init(): Promise<void> {
        // Idempotent: React 18 StrictMode (and remounts) call init() more than
        // once; without this guard each call would duplicate the module lists.
        if (Config.initPromise) {
            return Config.initPromise;
        }

        Config.initPromise = (async () => {
            // 1) Backend modules + this user's rights, from the server.
            await Auth.loadModules();
            Config.backendModules = Auth.getModules().map((m) => ({
                name: m.name,
                title: m.description || m.name,
                href: (m.endpoint || "/" + m.name).replace(/^\//, ""),
                description: m.description,
                modes: m.modes || [],
            }));
            const backendHrefs = new Set(Config.backendModules.map((m) => m.href));

            // 2) Frontend-only custom pages, from the static page scan. Skip any
            //    whose href a backend module already owns, and any the user may
            //    not access.
            const loaded = await import("../components/pages");
            Config.customPages = [];

            Object.keys(loaded).forEach((key) => {
                const Cls = (loaded as any)[key];
                if (!Cls || !("guid" in Cls)) return;

                const meta = new Cls({});
                const href: string = meta.getHref();

                // Home ("" href) is handled by the explicit "/" route.
                if (!href) return;
                // A backend module owns this route; don't double-register.
                if (backendHrefs.has(href)) return;

                const page: CustomPage = {
                    href,
                    name: key,
                    uuid: meta.getUUID(),
                    title: meta.getTitle(),
                    isPage: meta.isPageComponent(),
                    requiresAuth:
                        typeof meta.requiresAuthentication === "function"
                            ? meta.requiresAuthentication()
                            : false,
                    requiresAdmin:
                        typeof meta.requiresAdministration === "function"
                            ? meta.requiresAdministration()
                            : false,
                    component: Cls,
                    props: Cls.props || {},
                    extraRoutes: Cls.extraRoutes || [],
                    condition: typeof Cls.condition === "function" ? Cls.condition : undefined,
                };

                if (!Auth.canAccessModule(page)) return;
                Config.customPages.push(page);
            });
        })();

        return Config.initPromise;
    }

    /** Backend modules (rights-managed) the current user may access. */
    public static getModules(): BackendModuleEntry[] {
        return Config.backendModules;
    }

    public static getModule(name: string): BackendModuleEntry | undefined {
        return Config.backendModules.find((m) => m.name === name);
    }

    /** Frontend-only custom pages the current user may access. */
    public static getCustomPages(): CustomPage[] {
        return Config.customPages;
    }

    /** Custom pages shown in the "Pages" dropdown. */
    public static getPages(): CustomPage[] {
        return Config.customPages.filter((p) => p.isPage);
    }

    /** Custom pages shown as top-level nav items. */
    public static getNavPages(): CustomPage[] {
        return Config.customPages.filter((p) => !p.isPage);
    }
}