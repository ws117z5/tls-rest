// Client-side view of authentication state.
//
// The server injects the current auth status into the page shell as
// `window.__AUTH__` (see templates/index.gohtml, populated from the session in
// controllers.Index). This helper is the single place the frontend reads it, so
// menus and guards can react to it without each component re-deriving it.

declare global {
    interface Window {
        __AUTH__?: boolean;
    }
}

export default class Auth {
    /** True when the current visitor has an authenticated session. */
    static isAuthenticated(): boolean {
        return typeof window !== "undefined" && window.__AUTH__ === true;
    }
}