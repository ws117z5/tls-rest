import { useEffect, useState } from "react";
import axios from "axios";

// Pure-JS pages/modules get their HTML text from the html module: a row with a slug is published at /api/html/<slug>.
const cache = new Map<string, Promise<string>>();

// fetchHtml returns the html published under slug; a failed fetch is not cached, so a later call retries.
export function fetchHtml(slug: string): Promise<string> {
  let p = cache.get(slug);
  if (!p) {
    p = axios
      .get(`/api/html/${encodeURIComponent(slug)}`, { headers: { "X-Request-Type": "api" } })
      .then((res) => String(res.data?.html ?? ""));
    p.catch(() => cache.delete(slug));
    cache.set(slug, p);
  }
  return p;
}

// useHtml loads the html published under slug; html stays "" until it arrives, failed turns true if it can't be loaded.
export function useHtml(slug: string): { html: string; failed: boolean } {
  const [state, setState] = useState({ html: "", failed: false });
  useEffect(() => {
    let mounted = true;
    fetchHtml(slug)
      .then((html) => mounted && setState({ html, failed: false }))
      .catch(() => mounted && setState({ html: "", failed: true }));
    return () => {
      mounted = false;
    };
  }, [slug]);
  return state;
}
