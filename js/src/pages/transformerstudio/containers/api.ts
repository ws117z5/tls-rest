// Client for the model server's v1 API (see the API doc pasted into this feature's task), reached through this app's
// own backend (go/pages/transformerstudio/proxy.go) rather than directly — the model server may be anywhere on the
// LAN and has no CORS/auth story of its own, so the browser never talks to it.

const HDR = { "Content-Type": "application/json", "X-Request-Type": "api" } as const;

export class ApiError extends Error {
  status: number;
  constructor(message: string, status: number) {
    super(message);
    this.status = status;
  }
}

interface Reply {
  status: number;
  json?: any;
  blob?: Blob;
  headers: Headers;
}

// call is the one path every JSON-bodied endpoint goes through: POST /api/transformer-studio/proxy
// {server, method, path, body}. The reply mirrors whatever the model server actually returned — JSON for
// status/config/translate/..., a PNG blob for image endpoints — never assumed ahead of time.
async function call(server: string, method: string, path: string, body?: unknown): Promise<Reply> {
  const res = await fetch("/api/transformer-studio/proxy", {
    method: "POST",
    headers: HDR,
    body: JSON.stringify({ server, method, path, body: body === undefined ? undefined : body }),
  });
  const ct = res.headers.get("Content-Type") || "";
  if (ct.includes("application/json")) {
    const json = await res.json().catch(() => ({}));
    if (!res.ok) throw new ApiError(json?.error || res.statusText, json?.status || res.status);
    return { status: res.status, json, headers: res.headers };
  }
  const blob = await res.blob();
  if (!res.ok) throw new ApiError(res.statusText, res.status);
  return { status: res.status, blob, headers: res.headers };
}

const asJson = async (server: string, method: string, path: string, body?: unknown) =>
  (await call(server, method, path, body)).json;

// --- discovery / state --------------------------------------------------------

export const getStatus = (server: string) => asJson(server, "GET", "/api/v1/status");
export const getModels = (server: string) => asJson(server, "GET", "/api/v1/models");
export const selectModel = (server: string, name: string) => asJson(server, "POST", "/api/v1/models/select", { name });
export const createModel = (server: string, params: Record<string, unknown>) =>
  asJson(server, "POST", "/api/v1/models", params);
export const setConfig = (server: string, params: Record<string, unknown>) =>
  asJson(server, "POST", "/api/v1/config", params);
export const buildTokenizer = (server: string, params: Record<string, unknown>) =>
  asJson(server, "POST", "/api/v1/tokenizers", params);
export const trainStart = (server: string) => asJson(server, "POST", "/api/v1/train/start");
export const trainStop = (server: string) => asJson(server, "POST", "/api/v1/train/stop");
export const translate = (server: string, text: string) => asJson(server, "POST", "/api/v1/translate", { text });

// --- binary results (image / video) -------------------------------------------

export interface BlobResult {
  url: string;
  headers: Record<string, string>;
}

function toBlobResult(reply: Reply, wantedHeaders: string[]): BlobResult {
  if (!reply.blob) throw new ApiError("expected a binary response", reply.status);
  const headers: Record<string, string> = {};
  for (const h of wantedHeaders) {
    const v = reply.headers.get(h);
    if (v) headers[h] = v;
  }
  return { url: URL.createObjectURL(reply.blob), headers };
}

const IMAGE_HEADERS = ["X-Seed", "X-Generation-Ms"];
const VIDEO_HEADERS = ["X-Seed", "X-Generation-Ms", "X-Frames"];
const EXPRESSION_HEADERS = ["X-Face-Found", "X-Edit-Ms"];

export async function generateImage(server: string, params: Record<string, unknown>): Promise<BlobResult> {
  return toBlobResult(await call(server, "POST", "/api/v1/images/generate", params), IMAGE_HEADERS);
}

export async function enhanceImage(server: string, image: string, params: Record<string, unknown>): Promise<BlobResult> {
  return toBlobResult(await call(server, "POST", "/api/v1/enhance", { image, params }), ["X-Enhance-Ms"]);
}

export async function enhanceVideo(
  server: string,
  file: File,
  params: Record<string, unknown>,
  maxFrames: number,
  varyNoise: boolean
): Promise<BlobResult> {
  const form = new FormData();
  form.append("video", file);
  form.append("params", JSON.stringify(params));
  form.append("max_frames", String(maxFrames));
  form.append("vary_noise", String(varyNoise));
  const res = await fetch(`/api/transformer-studio/enhance-video?server=${encodeURIComponent(server)}`, {
    method: "POST",
    headers: { "X-Request-Type": "api" },
    body: form,
  });
  const blob = await res.blob();
  if (!res.ok) {
    const text = await blob.text().catch(() => "");
    let message = res.statusText;
    try {
      message = JSON.parse(text)?.error || message;
    } catch {
      // not JSON
    }
    throw new ApiError(message, res.status);
  }
  return toBlobResult({ status: res.status, blob, headers: res.headers }, ["X-Enhance-Ms", "X-Frames"]);
}

export const getExpressionInfo = (server: string) => asJson(server, "GET", "/api/v1/expression");

export async function editExpression(
  server: string,
  image: string,
  params: Record<string, number>
): Promise<BlobResult> {
  return toBlobResult(await call(server, "POST", "/api/v1/expression", { image, ...params }), EXPRESSION_HEADERS);
}

export const getVideoInfo = (server: string) => asJson(server, "GET", "/api/v1/video");

export async function generateVideo(server: string, params: Record<string, unknown>): Promise<BlobResult> {
  return toBlobResult(await call(server, "POST", "/api/v1/video", params), VIDEO_HEADERS);
}

// --- live events ---------------------------------------------------------------

export type StudioEvent = { type: string; [key: string]: any };

// subscribeEvents opens the server-sent /events stream through our own proxy (so the browser only ever talks to this
// app's origin) and returns a disposer. EventSource retries on its own if the connection drops.
export function subscribeEvents(server: string, onEvent: (e: StudioEvent) => void): () => void {
  const es = new EventSource(`/api/transformer-studio/events?server=${encodeURIComponent(server)}`);
  es.onmessage = (m) => {
    try {
      onEvent(JSON.parse(m.data));
    } catch {
      // ignore a malformed event rather than tearing down the stream
    }
  };
  return () => es.close();
}
