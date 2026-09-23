// Bus carrying one click's combined DB query count/duration (admin only),
// batched across BATCH_WINDOW_MS since one click can fire several requests.
// Each response carries its own totals as headers (no follow-up request, so
// nothing can race it) — see querystats.go/middleware.go for the server side.
// Read by <QueryStatsFooter>.

import axios from "axios";
import Auth from "@controllers/auth";

export interface QueryStatsSnapshot {
  paths: string[];
  count: number;
  ms: number;
}

type Listener = (snap: QueryStatsSnapshot) => void;

const BATCH_WINDOW_MS = 250;

class QueryStatsBus {
  private listeners: Listener[] = [];
  private pending: QueryStatsSnapshot | null = null;
  private timer: ReturnType<typeof setTimeout> | null = null;

  subscribe(fn: Listener): () => void {
    this.listeners.push(fn);
    return () => {
      this.listeners = this.listeners.filter((l) => l !== fn);
    };
  }

  // Folds one request's stats into the current batch, resetting the flush timer.
  add(path: string, count: number, ms: number) {
    if (!this.pending) this.pending = { paths: [], count: 0, ms: 0 };
    if (this.pending.paths.indexOf(path) === -1) this.pending.paths.push(path);
    this.pending.count += count;
    this.pending.ms += ms;
    if (this.timer != null) clearTimeout(this.timer);
    this.timer = setTimeout(() => this.flush(), BATCH_WINDOW_MS);
  }

  private flush() {
    this.timer = null;
    const snap = this.pending;
    this.pending = null;
    if (!snap) return;
    this.listeners.forEach((l) => l(snap));
  }
}

const QueryStats = new QueryStatsBus();
export default QueryStats;

// Server sets these on the *same* response once it knows the totals — no
// follow-up request, so there's no window for it to 404 or arrive stale.
const REQUEST_HEADER = "X-Query-Stats";
const COUNT_HEADER = "x-query-stats-count";
const MILLIS_HEADER = "x-query-stats-ms";
let installed = false;

// Tags every axios call so the server knows to instrument it (server still
// requires an admin session regardless), then reads the totals straight off
// each response. Call once at app startup (see app.tsx).
export function installQueryStatsRefresh(): void {
  if (installed) return;
  installed = true;

  axios.interceptors.request.use((config: any) => {
    if (Auth.isAdmin()) {
      config.headers = config.headers || {};
      config.headers[REQUEST_HEADER] = "1";
    }
    return config;
  });

  const record = (res: any) => {
    const count = res?.headers?.[COUNT_HEADER];
    if (count === undefined) return;
    const path = res?.config?.url || "";
    QueryStats.add(path, Number(count), Number(res.headers[MILLIS_HEADER] || 0));
  };

  axios.interceptors.response.use(
    (res) => {
      record(res);
      return res;
    },
    (err) => {
      if (err?.response) record(err.response);
      return Promise.reject(err);
    }
  );
}
