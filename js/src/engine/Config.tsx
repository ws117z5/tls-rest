import Auth, { MenuEntry, isModuleEntry, BackendPage } from "@controllers/auth";

// A resolved, renderable menu item. Modules route to the generic ModulePage;
// pages route to their barrel component. `path` is the full route/link path.
export interface MenuItem {
    kind: "module" | "page";
    key: string;        // module name or page id
    name: string;       // = key (app.tsx module route reads .name)
    title: string;
    href: string;       // path segment, no leading slash
    path: string;       // full path, e.g. "/posts" or "/pages/netmapper"
    modes: string[];    // modules only
    icon?: string;      // menu icon URL
    isPage: boolean;    // pages: from barrel isPageComponent; modules: false
    component?: any;     // pages only
    props?: Record<string, any>;
    extraRoutes?: Array<{ href: string; component: any }>;
}

type BarrelEntry = { component: any; title: string; isPage: boolean; submenu: string };

export default class Config {
    private static head: MenuItem[] = [];
    private static submenus: Record<string, MenuItem[]> = {};
    private static modules: MenuItem[] = [];  // flattened, for routing
    private static pages: MenuItem[] = [];    // flattened (with component), for routing
    private static initPromise: Promise<void> | null = null;
    public static serverURL = window.location.origin + "/";

    static init(): Promise<void> {
        // Idempotent: StrictMode / remounts call init() more than once.
        if (Config.initPromise) return Config.initPromise;

        Config.initPromise = (async () => {
            // 1) The complete, privilege-filtered menu from the server.
            await Auth.loadMenu();

            // 2) Barrel of page components, keyed by href (provides the React
            //    component to render each page; the server only sends metadata).
            const barrel = await Config.loadBarrel();

            const covered = new Set<string>();

            const toItem = (entry: MenuEntry): MenuItem => {
                if (isModuleEntry(entry)) {
                    const href = (entry.endpoint || "/" + entry.name).replace(/^\//, "");
                    covered.add(href);
                    return {
                        kind: "module",
                        key: entry.name,
                        name: entry.name,
                        title: entry.description || entry.name,
                        href,
                        path: "/" + href,
                        modes: entry.modes || [],
                        icon: entry.icon,
                        isPage: false,
                    };
                }
                const p = entry as BackendPage;
                const href = (p.endpoint || "/" + p.id).replace(/^\//, "");
                covered.add(href);
                const b = barrel[href] || barrel[p.id];
                const isPage = b ? b.isPage : true;
                return {
                    kind: "page",
                    key: p.id,
                    name: p.id,
                    title: p.name,
                    href,
                    path: "/" + (isPage ? "pages/" : "") + href,
                    modes: [],
                    icon: p.icon,
                    isPage,
                    component: b?.component,
                    props: b?.component?.props || {},
                    extraRoutes: b?.component?.extraRoutes || [],
                };
            };

            Config.head = Auth.getHead().map(toItem);
            Config.submenus = {};
            const subs = Auth.getSubmenus();
            Object.keys(subs).forEach((title) => {
                Config.submenus[title] = subs[title].map(toItem);
            });

            // 3) Pure-frontend pages: in the barrel but not named by the server
            //    (public tools with no backend page). Keep them reachable —
            //    grouped by the component's own submenu, or head if none.
            Object.keys(barrel).forEach((href) => {
                if (covered.has(href)) return;
                const b = barrel[href];
                if (!b.isPage) return;
                const item: MenuItem = {
                    kind: "page",
                    key: href,
                    name: href,
                    title: b.title,
                    href,
                    path: "/pages/" + href,
                    modes: [],
                    isPage: true,
                    component: b.component,
                    props: b.component?.props || {},
                    extraRoutes: b.component?.extraRoutes || [],
                };
                if (b.submenu) {
                    (Config.submenus[b.submenu] = Config.submenus[b.submenu] || []).push(item);
                } else {
                    Config.head.push(item);
                }
            });

            // 4) Flatten for routing.
            const all = [...Config.head, ...Object.values(Config.submenus).flat()];
            Config.modules = all.filter((i) => i.kind === "module");
            Config.pages = all.filter((i) => i.kind === "page" && i.component);
        })();

        return Config.initPromise;
    }

    private static async loadBarrel(): Promise<Record<string, BarrelEntry>> {
        const [appPages, enginePages] = await Promise.all([
            import("../components/pages"),
            import("@engine/pages"),
        ]);
        const map: Record<string, BarrelEntry> = {};
        const scan = (loaded: Record<string, any>) => {
            Object.keys(loaded).forEach((key) => {
                const Cls = (loaded as any)[key];
                if (!Cls || !("guid" in Cls)) return;
                const meta = new Cls({});
                const href: string = meta.getHref();
                if (map[href]) return;
                map[href] = {
                    component: Cls,
                    title: meta.getTitle(),
                    isPage: typeof meta.isPageComponent === "function" ? meta.isPageComponent() : true,
                    submenu: typeof meta.getSubmenu === "function" ? meta.getSubmenu() : "",
                };
            });
        };
        scan(appPages as Record<string, any>);
        scan(enginePages as Record<string, any>);
        return map;
    }

    // --- accessors ---
    /** Top-level menu items (modules + pages with no submenu). */
    public static getHead(): MenuItem[] { return Config.head; }
    /** Submenu groups, keyed by title. */
    public static getSubmenus(): Record<string, MenuItem[]> { return Config.submenus; }

    /** Module items (for ModulePage routing). app.tsx reads href/name/title/modes. */
    public static getModules(): MenuItem[] { return Config.modules; }
    public static getModule(name: string): MenuItem | undefined {
        return Config.modules.find((m) => m.key === name);
    }
    /** Page items with a component (for page routing). app.tsx reads href/isPage/component/extraRoutes. */
    public static getCustomPages(): MenuItem[] { return Config.pages; }
    public static getPages(): MenuItem[] { return Config.pages; }
}