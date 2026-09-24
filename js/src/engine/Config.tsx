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
    customViews?: Record<string, Record<string, string>>; // mode -> {viewName: label}
    configAffecting?: boolean; // modules only; a write here should reload AppConfig
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
    private static pagesSubscribers: Array<() => void> = [];
    public static serverURL = window.location.origin + "/";

    // init resolves as soon as the server menu is known — modules (the bulk of
    // the app) render immediately via the generic ModulePage, which needs no
    // page component at all. Page components (barrel scan below) are heavy
    // (three.js, opencv, graphviz, …) and load in the background afterward;
    // subscribe via onPagesReady to know when they land.
    static init(): Promise<void> {
        // Idempotent: StrictMode / remounts call init() more than once.
        if (Config.initPromise) return Config.initPromise;

        Config.initPromise = (async () => {
            await Auth.loadMenu();
            await AppConfig.load();
            Config.rebuild({});

            Config.loadBarrel().then((barrel) => {
                Config.rebuild(barrel);
                Config.pagesSubscribers.forEach((cb) => cb());
            });
        })();

        return Config.initPromise;
    }

    // onPagesReady notifies once page components (barrel) have loaded and
    // Config's menu/routing lists were rebuilt with them.
    static onPagesReady(cb: () => void): () => void {
        Config.pagesSubscribers.push(cb);
        return () => {
            Config.pagesSubscribers = Config.pagesSubscribers.filter((c) => c !== cb);
        };
    }

    // rebuild derives head/submenus/modules/pages from the server menu plus
    // barrel (page components discovered from the filesystem, keyed by href —
    // the server only sends page metadata). Called once with an empty barrel
    // for the immediate render, then again once the real barrel resolves.
    private static rebuild(barrel: Record<string, BarrelEntry>) {
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
                    customViews: entry.customViews,
                    configAffecting: entry.configAffecting,
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
                customViews: p.customViews,
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

        // Pure-frontend pages: in the barrel but not named by the server
        // (public tools with no backend page). Keep them reachable — grouped
        // by the component's own submenu, or head if none.
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

        const all = [...Config.head, ...Object.values(Config.submenus).flat()];
        Config.modules = all.filter((i) => i.kind === "module");
        Config.pages = all.filter((i) => i.kind === "page" && i.component);
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

        // Fetch every matched module in parallel — the sequential "for...await"
        // this replaced serialized what should be independent network fetches,
        // one file behind the next (irrelevant now anyway since this whole scan
        // runs in the background, off the critical render path — see init()).
        const pairs: Array<{ ctx: __WebpackModuleApi.RequireContext; key: string }> = [];
        contexts.forEach((ctx) => ctx.keys().forEach((key) => pairs.push({ ctx, key })));
        const mods = await Promise.all(
            pairs.map(({ ctx, key }) => ctx(key).catch(() => null)) // a module that throws on import is not a usable page
        );

        const map: Record<string, BarrelEntry> = {};
        mods.forEach((mod) => {
            if (!mod) return;
            Object.keys(mod as Record<string, any>).forEach((exportName) => {
                const Cls = (mod as Record<string, any>)[exportName];
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
        });
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