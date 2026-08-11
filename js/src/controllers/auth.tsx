// Client-side view of authentication + authorization state.
//
// The server still injects basic session status into the page shell:
//   window.__AUTH__  - boolean, is there an authenticated session
//   window.__ADMIN__ - boolean, is the user an administrator
//
// The module list and this user's per-module rights come from GET /api/modules
// (rights-aware; see controllers.ModulesAPI). That response only contains the
// modules the user may access, each with the modes they may perform, so the
// frontend renders menu items and gates modes directly from it.

import axios from "axios";

export interface AccessibleModule {
    href?: string;
    requiresAuth?: boolean;
    requiresAdmin?: boolean;
}

// One backend module as reported by /api/modules.
export interface BackendModule {
    name: string;
    description: string;
    endpoint: string;   // e.g. "/posts"
    modes: string[];    // subset of ["list","view","create","edit","delete"]
}

declare global {
    interface Window {
        __AUTH__?: boolean;
        __ADMIN__?: boolean;
    }
}

export default class Auth {
    private static modules: BackendModule[] = [];
    private static admin: boolean = false;
    private static loaded: boolean = false;

    /**
     * Fetch the current user's accessible modules + rights from the backend.
     * Called once during Config.init(). Failures degrade to "no modules".
     */
    static async loadModules(): Promise<void> {
        try {
            const res = await axios.get("/api/modules");
            Auth.modules = Array.isArray(res.data?.modules) ? res.data.modules : [];
            Auth.admin = res.data?.isAdmin === true;
        } catch (err) {
            console.error("Failed to load modules:", err);
            Auth.modules = [];
            Auth.admin = false;
        } finally {
            Auth.loaded = true;
        }
    }

    static modulesLoaded(): boolean {
        return Auth.loaded;
    }

    /** All backend modules the current user may access. */
    static getModules(): BackendModule[] {
        return Auth.modules;
    }

    static getModule(name: string): BackendModule | undefined {
        return Auth.modules.find((m) => m.name === name);
    }

    /** Modes the current user may perform on a module (empty if none/unknown). */
    static moduleModes(name: string): string[] {
        return Auth.getModule(name)?.modes || [];
    }

    /** Whether the current user may perform a given mode on a module. */
    static canMode(name: string, mode: string): boolean {
        return Auth.moduleModes(name).indexOf(mode) !== -1;
    }

    /** True when the current visitor has an authenticated session. */
    static isAuthenticated(): boolean {
        return typeof window !== "undefined" && window.__AUTH__ === true;
    }

    /** True when the current visitor is an administrator. */
    static isAdmin(): boolean {
        if (Auth.admin) return true;
        return typeof window !== "undefined" && window.__ADMIN__ === true;
    }

    /**
     * Access check for frontend-only custom pages (which are not backend
     * modules and therefore not in /api/modules). Backend modules are already
     * filtered by the server, so their presence in getModules() is the check.
     */
    static canAccessModule(module: AccessibleModule): boolean {
        if (module.requiresAdmin && !Auth.isAdmin()) return false;
        if (module.requiresAuth && !Auth.isAuthenticated()) return false;
        return true;
    }
}