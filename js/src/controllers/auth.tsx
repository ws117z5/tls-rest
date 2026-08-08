// Client-side view of authentication + authorization state.
//
// The server injects the current user's status into the page shell:
//   window.__AUTH__     - boolean, is there an authenticated session
//   window.__ADMIN__    - boolean, is the user an administrator
//   window.__MANAGED__  - string[], module hrefs the backend governs (rights-controlled)
//   window.__MODULES__  - string[], the subset of managed modules this user may access
//
// A managed module that is absent from __MODULES__ is one the user has no rights
// to; the frontend does not load it. Modules the backend does not govern
// (custom/frontend pages) are not in __MANAGED__ and are always loadable
// (subject to requiresAuth / requiresAdmin).

export interface AccessibleModule {
    href?: string;
    requiresAuth?: boolean;
    requiresAdmin?: boolean;
}

declare global {
    interface Window {
        __AUTH__?: boolean;
        __ADMIN__?: boolean;
        __MANAGED__?: string[];
        __MODULES__?: string[];
    }
}

export default class Auth {
    /** True when the current visitor has an authenticated session. */
    static isAuthenticated(): boolean {
        return typeof window !== "undefined" && window.__AUTH__ === true;
    }

    /** True when the current visitor is an administrator. */
    static isAdmin(): boolean {
        return typeof window !== "undefined" && window.__ADMIN__ === true;
    }

    /** Module hrefs the backend governs (rights-controlled). */
    static managedModules(): string[] {
        return (typeof window !== "undefined" && window.__MANAGED__) || [];
    }

    /** Module hrefs available to the current user. */
    static availableModules(): string[] {
        return (typeof window !== "undefined" && window.__MODULES__) || [];
    }

    /**
     * Whether the current user may load a module. A managed module must be in the
     * available set; unmanaged (custom) modules are allowed. requiresAuth /
     * requiresAdmin flags are always enforced.
     */
    static canAccessModule(module: AccessibleModule): boolean {
        if (module.requiresAdmin && !Auth.isAdmin()) return false;
        if (module.requiresAuth && !Auth.isAuthenticated()) return false;

        const href = module.href;
        if (href && Auth.managedModules().indexOf(href) !== -1) {
            // Backend-governed: only if the backend reported it as available.
            return Auth.availableModules().indexOf(href) !== -1;
        }

        return true;
    }
}