// A tiny app-wide notification bus. Anything can push a notification and the
// <Toasts> container (mounted once at the app root) renders them. Used by the
// axios error interceptor to surface {status, message, log_id} responses, and
// available for success/info messages elsewhere.

export type NotifyKind = "error" | "success" | "info" | "warning";

export interface Notification {
  id: number;
  kind: NotifyKind;
  message: string;
  logId?: string;
  timeout?: number; // ms before auto-dismiss; 0/undefined = sticky
}

type Listener = (n: Notification) => void;

class NotifyBus {
  private listeners: Listener[] = [];
  private seq = 0;

  subscribe(fn: Listener): () => void {
    this.listeners.push(fn);
    return () => {
      this.listeners = this.listeners.filter((l) => l !== fn);
    };
  }

  private emit(kind: NotifyKind, message: string, logId?: string, timeout?: number) {
    const n: Notification = { id: ++this.seq, kind, message, logId, timeout };
    this.listeners.forEach((l) => l(n));
  }

  // Errors linger longer and carry the log id for later lookup in the logs module.
  error(message: string, logId?: string) {
    this.emit("error", message, logId, 10000);
  }
  success(message: string) {
    this.emit("success", message, undefined, 4000);
  }
  info(message: string) {
    this.emit("info", message, undefined, 5000);
  }
  warning(message: string) {
    this.emit("warning", message, undefined, 7000);
  }
}

const Notify = new NotifyBus();
export default Notify;