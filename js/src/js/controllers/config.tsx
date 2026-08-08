import { Component } from "react";

import modulesConfig from '../../modules.json'
import PageComponent from "./PageComponent";
import Auth from "./auth";

type Submodule = {
    href: string;
    component: Component;
    props?: Record<string, any>;
}

type Module = {
    href: string;
    name: string;
    title: string;
    uuid: string;
    isPage?: boolean;
    requiresAuth?: boolean;
    requiresAdmin?: boolean;
    component: Component;
    props?: Record<string, any>;
    extraRoutes: Submodule[];
    fieldset?: any;
}



export default class Config {
    private static modules: Module[] = [];
    private static moduleMap = new Map<string, number>();
    private static initPromise: Promise<void> | null = null;
    public static len: number = 0;
    public static serverURL = window.location.origin + "/"

    static init(): Promise<void> {
        // Idempotent: return the in-flight (or already-completed) initialization.
        // Without this guard, React 18 StrictMode — and any remount — calls init()
        // more than once, and because each call pushes onto the static module list,
        // every module (and therefore every nav-bar item) gets duplicated.
        if (Config.initPromise) {
            return Config.initPromise;
        }

        Config.initPromise = import("../components/pages").then((loaded) => {

            // Reset first so a run never accumulates duplicates.
            Config.modules = [];
            Config.moduleMap.clear();
            Config.len = 0;

            Object.keys(loaded).forEach((key) => {

                const Cls = (loaded as any)[key];

                if (Cls && 'guid' in Cls) {

                    const meta = new Cls({});

                    let _module: Module = {
                        href: meta.getHref(),
                        name: key,
                        uuid: meta.getUUID(),
                        title: meta.getTitle(),
                        isPage: meta.isPageComponent(),
                        requiresAuth: typeof meta.requiresAuthentication === "function" ? meta.requiresAuthentication() : false,
                        requiresAdmin: typeof meta.requiresAdministration === "function" ? meta.requiresAdministration() : false,
                        component: Cls,
                        props: Cls.props || {},
                        extraRoutes: Cls.extraRoutes ? Cls.extraRoutes : [],
                    };

                    // Don't load modules the current user has no access to: a
                    // backend-governed module absent from the available list, or a
                    // module gated by requiresAuth/requiresAdmin. Absent modules
                    // are never registered — no menu item, no route, no component.
                    if (!Auth.canAccessModule(_module)) {
                        return;
                    }

                    let idx: number = Config.modules.push(_module)
                    Config.moduleMap.set(key, idx-1);
                    Config.len++;

                }
            });
        });

        return Config.initPromise;
    }


    public static get(name: string): Module | null {
        let idx = Config.moduleMap.get(name);
        return idx !== undefined ? Config.modules[idx] : null;
    }

    public static getAll(): Module[] | null {
        return Config.modules;
    }

    public static getPages(): Module[] | null {
        return Config.modules.filter(module => { return module.isPage == true; });
    }

}