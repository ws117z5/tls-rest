import axios from "axios";
import Notify from "../containers/Notify";

// installHttpErrorToasts wires a single global axios response interceptor that
// turns every failed request into an error toast. It reads the structured error
// body the backend now returns — { status, message, log_id } — and falls back
// gracefully for plain-text or network errors. Call once at app startup.
let installed = false;

export function installHttpErrorToasts(): void {
  if (installed) return;
  installed = true;

  axios.interceptors.response.use(
    (res: any) => res,
    (error: any) => {
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