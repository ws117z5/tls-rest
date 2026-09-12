import AppConfig from "@engine/controllers/Appconfig";
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
    // Column a module's records are addressed by ("id" unless the module sets a
    // different KeyField, e.g. papers uses "uuid"). Modules only.
    keyField?: string;
    component?: any;     // pages only
    props?: Record<string, any>;
    extraRoutes?: Array<{ href: string; component: any }>;
}

type BarrelEntry = { component: any; title: string; isPage: boolean; submenu: string; icon: string; requiresAuth: boolean; requiresAdmin: boolean };

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
            await AppConfig.load();

            // 2) Page components discovered from the filesystem, keyed by href
            //    (the server only sends page metadata; this supplies the React
            //    component to render each one).
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
                        keyField: entry.key_field || "id",
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
                // Gate by the component's own auth requirements. This prevents a
                // backend page the server hid for this user (e.g. an auth-only
                // page for a guest) from reappearing here and 401-ing on click.
                if (b.requiresAdmin && !Auth.isAdmin()) return;
                if (b.requiresAuth && !Auth.isAuthenticated()) return;
                const item: MenuItem = {
                    kind: "page",
                    key: href,
                    name: href,
                    title: b.title,
                    href,
                    path: "/pages/" + href,
                    modes: [],
                    icon: b.icon,
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

    // loadBarrel discovers page components from the filesystem, keyed by the
    // href each one declares. Any PageComponent subclass exported (default or
    // named) from pages/<dir>/<Name>.tsx — or a flat pages/<Name>.tsx — is picked
    // up automatically, mirroring how module view overrides are found
    // (controllers/registry.ts). Adding a page never means editing a list here.
    //   ./pages            engine pages (this file lives in engine/)
    //   ../pages           app pages
    //   ./components/pages  legacy app page components
    private static async loadBarrel(): Promise<Record<string, BarrelEntry>> {
        // pages/<Dir>/<PascalName>.tsx or a flat pages/<PascalName>.tsx — one
        // level deep, so sub-components (containers/, controllers/, lowercase
        // helpers) are ignored.
        //
        // mode:"lazy-once" keeps every page component (and its deps — three.js,
        // opencv, graphviz, …) OUT of the entry chunk: they go into one shared
        // chunk fetched here during init, exactly like the old dynamic-import
        // barrel. Path and RegExp MUST be inline literals on
        // `import.meta.webpackContext(` — webpack only static-analyses that exact
        // call shape, not an aliased variable.
        const contexts: __WebpackModuleApi.RequireContext[] = [
            (import.meta as any).webpackContext("./pages", {
                recursive: true,
                mode: "lazy-once",
                chunkName: "pages",
                regExp: /^\.\/(?:[^/]+\/)?[A-Z][A-Za-z0-9]*\.tsx$/,
            }),
            (import.meta as any).webpackContext("../pages", {
                recursive: true,
                mode: "lazy-once",
                chunkName: "pages",
                regExp: /^\.\/(?:[^/]+\/)?[A-Z][A-Za-z0-9]*\.tsx$/,
            }),
            (import.meta as any).webpackContext("../components/pages", {
                recursive: true,
                mode: "lazy-once",
                chunkName: "pages",
                regExp: /^\.\/(?:[^/]+\/)?[A-Z][A-Za-z0-9]*\.tsx$/,
            }),
        ];

        const map: Record<string, BarrelEntry> = {};
        for (const ctx of contexts) {
            for (const key of ctx.keys()) {
                let mod: Record<string, any>;
                try {
                    mod = (await ctx(key)) as Record<string, any>;
                } catch {
                    continue; // a module that throws on import is not a usable page
                }
                Object.keys(mod).forEach((exportName) => {
                    const Cls = mod[exportName];
                    // PageComponent subclasses carry the inherited static guid().
                    if (typeof Cls !== "function" || !("guid" in Cls)) return;
                    let inst: any;
                    try {
                        inst = new Cls({});
                    } catch {
                        return;
                    }
                    if (typeof inst.getHref !== "function") return;
                    const href: string = inst.getHref();
                    // "" is valid — it is the home page's href. Reject only a
                    // non-string or a duplicate.
                    if (typeof href !== "string" || map[href] !== undefined) return;
                    map[href] = {
                        component: Cls,
                        title: typeof inst.getTitle === "function" ? inst.getTitle() : href,
                        isPage: typeof inst.isPageComponent === "function" ? inst.isPageComponent() : true,
                        submenu: typeof inst.getSubmenu === "function" ? inst.getSubmenu() : "",
                        icon: typeof inst.getIcon === "function" ? inst.getIcon() : "",
                        requiresAuth: typeof inst.requiresAuthentication === "function" ? inst.requiresAuthentication() : false,
                        requiresAdmin: typeof inst.requiresAdministration === "function" ? inst.requiresAdministration() : false,
                    };
                });
            }
        }
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