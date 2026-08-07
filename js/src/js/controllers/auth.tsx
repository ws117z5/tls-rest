// Client-side view of authentication state.
//
// The server injects the current auth status into the page shell as
// `window.__AUTH__` and `window.__ADMIN__` (see templates/index.gohtml,
// populated from the session in controllers.Index). This helper is the single
// place the frontend reads them, so menus and guards can react without each
// component re-deriving it.

declare global {
    interface Window {
        __AUTH__?: boolean;
        __ADMIN__?: boolean;
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
}