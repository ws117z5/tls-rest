import axios from "axios";
import Notify from "../containers/Notify";

// installHttpErrorToasts wires a single global axios response interceptor that
// turns every failed request into an error toast. It reads the structured error
// body the backend now returns — { status, message, log_id } — and falls back
// gracefully for plain-text or network errors. Call once at app startup.
let installed = false;
// Set on the first 401 seen outside login/register/oauth, so a burst of concurrent/polling
// requests (e.g. the Actions page's 2s poll) failing together shows exactly one toast and
// triggers exactly one redirect instead of stacking one per request. Never reset — the page
// navigation below replaces this whole script anyway.
let redirectingUnauthorized = false;

export function installHttpErrorToasts(): void {
  if (installed) return;
  installed = true;

  axios.interceptors.response.use(
    (res: any) => res,
    (error: any) => {
      const status = error?.response?.status;
      const url: string = error?.config?.url || "";
      // A 401 here is also the normal "wrong password"/"oauth failed" response for these
      // specific endpoints (see login.go) — that's a form-level error the login page already
      // displays inline, not a dead session to redirect away from.
      const isAuthEndpoint = url.includes("/api/login") || url.includes("/api/register") || url.includes("/api/auth/oauth");

      if (status === 401 && !isAuthEndpoint) {
        if (!redirectingUnauthorized) {
          redirectingUnauthorized = true;
          Notify.error("Your session has expired. Redirecting to the home page…");
          window.location.href = "/";
        }
        return Promise.reject(error);
      }

      let message = "Request failed";
      let logId: string | undefined;

      const data = error?.response?.data;
      if (data && typeof data === "object") {
        if (typeof data.message === "string" && data.message) message = data.message;
        if (typeof data.log_id === "string" && data.log_id) logId = data.log_id;
      } else if (typeof data === "string" && data.trim()) {
        message = data.trim();
      } else if (error?.message) {
        message = error.message;
      }

      // 304 (fieldset cache hit) is handled by validateStatus elsewhere and
      // won't reach here; anything that does is a genuine failure.
      Notify.error(message, logId);
      return Promise.reject(error);
    }
  );
}