// Client-side view of the menu + per-module rights.
//
// GET /api/modules now returns the complete, already-privilege-filtered menu:
//
//   { head: [ ...entries... ], submenus: { "<title>": [ ...entries... ] } }
//
// Each entry is either a MODULE (has `modes`) or a PAGE (has `id`). The server
// has already filtered by the user's privileges, so there is no isAdmin / per-
// entry requiresAuth/requiresAdmin in the response and the client does not gate.
//
// window.__AUTH__ / window.__ADMIN__ are still injected in the page shell and
// used only as a coarse fallback for direct-URL access decisions.

import axios from "axios";

// A module entry from the menu.
export interface BackendModule {
    name: string;        // module key (rights/routing)
    description: string; // human label
    endpoint: string;    // e.g. "/posts"
    modes: string[];     // subset of ["list","view","create","edit","delete"]
    icon?: string;       // menu icon URL (e.g. /image/<uuid>)
}

// A page entry from the menu.
export interface BackendPage {
    id: string;
    name: string;
    endpoint: string;    // e.g. "/netmapper"
    icon?: string;       // menu icon URL
}

// Identity for the menu (login/logout swap + avatar).
export interface MenuUser {
    authenticated: boolean;
    avatar?: string;
}

export type MenuEntry = BackendModule | BackendPage;

export function isModuleEntry(e: MenuEntry): e is BackendModule {
    return Array.isArray((e as BackendModule).modes);
}
export function isPageEntry(e: MenuEntry): e is BackendPage {
    return typeof (e as BackendPage).id === "string" && !isModuleEntry(e);
}

declare global {
    interface Window {
        __AUTH__?: boolean;
        __ADMIN__?: boolean;
    }
}

export default class Auth {
    private static head: MenuEntry[] = [];
    private static submenus: Record<string, MenuEntry[]> = {};
    private static user: MenuUser = { authenticated: false };
    private static loaded: boolean = false;

    /**
     * Fetch the current user's menu from the backend. Called once during
     * Config.init(). Failures degrade to an empty menu.
     */
    static async loadMenu(): Promise<void> {
        try {
            const res = await axios.get("/api/modules");
            Auth.head = Array.isArray(res.data?.head) ? res.data.head : [];
            Auth.submenus =
                res.data?.submenus && typeof res.data.submenus === "object"
                    ? res.data.submenus
                    : {};
            Auth.user =
                res.data?.user && typeof res.data.user === "object"
                    ? { authenticated: res.data.user.authenticated === true, avatar: res.data.user.avatar }
                    : { authenticated: false };
        } catch (err) {
            console.error("Failed to load menu:", err);
            Auth.head = [];
            Auth.submenus = {};
            Auth.user = { authenticated: false };
        } finally {
            Auth.loaded = true;
        }
    }

    static menuLoaded(): boolean {
        return Auth.loaded;
    }

    static getHead(): MenuEntry[] {
        return Auth.head;
    }
    static getSubmenus(): Record<string, MenuEntry[]> {
        return Auth.submenus;
    }

    /** Every entry (head + all submenus), flattened. */
    static allEntries(): MenuEntry[] {
        const subs = Object.values(Auth.submenus).flat();
        return [...Auth.head, ...subs];
    }

    /** All module entries the user may access, flattened across the menu. */
    static getModules(): BackendModule[] {
        return Auth.allEntries().filter(isModuleEntry);
    }

    static getModule(name: string): BackendModule | undefined {
        return Auth.getModules().find((m) => m.name === name);
    }

    /** Modes the current user may perform on a module (empty if none/unknown). */
    static moduleModes(name: string): string[] {
        return Auth.getModule(name)?.modes || [];
    }

    /** Whether the current user may perform a given mode on a module. */
    static canMode(name: string, mode: string): boolean {
        return Auth.moduleModes(name).indexOf(mode) !== -1;
    }

    // Identity comes from the server menu response (see /api/modules "user").
    static isAuthenticated(): boolean {
        return Auth.user.authenticated === true;
    }
    /** Avatar URL when the user has one, else "" (used in the menu). */
    static getAvatar(): string {
        return Auth.user.avatar || "";
    }
    static isAdmin(): boolean {
        return typeof window !== "undefined" && window.__ADMIN__ === true;
    }

    /** Clear the session server-side, then reload to reset the SPA. */
    static async logout(): Promise<void> {
        try {
            await axios.post("/api/logout");
        } catch (err) {
            console.error("Logout failed:", err);
        }
        window.location.href = "/";
    }
}