import axios from "axios";

// Site i18n. Strings double as their own translation key (their English text),
// so components don't need a separate key-naming scheme. A string not yet in
// the local dict falls back to English immediately and is reported to the
// server in a debounced batch, which seeds it into the translations table
// (see go/engine/modules/translations) the first time it's ever seen.

type Dict = Record<string, string>;

const STORAGE_LOCALE_KEY = "locale";
// v2: earlier versions persisted the English fallback returned before a real
// translation existed, and never rechecked it — the key is versioned so a
// stale v1 cache (stuck showing English forever) is simply abandoned rather
// than needing a manual localStorage clear.
const dictStorageKey = (loc: string) => `i18n:v2:${loc}`;

function detectLocale(): string {
  try {
    const saved = localStorage.getItem(STORAGE_LOCALE_KEY);
    if (saved) return saved;
  } catch { /* ignore */ }
  // userLanguage is legacy IE/old-Edge; language is missing only in that case.
  const browserLocale = navigator.language || (navigator as any).userLanguage || "";
  return browserLocale.toLowerCase().startsWith("ru") ? "ru" : "en";
}

function loadCachedDict(loc: string): Dict {
  try {
    const raw = localStorage.getItem(dictStorageKey(loc));
    return raw ? JSON.parse(raw) : {};
  } catch {
    return {};
  }
}

function saveCachedDict(loc: string, d: Dict) {
  try {
    localStorage.setItem(dictStorageKey(loc), JSON.stringify(d));
  } catch { /* ignore (private mode, quota, ...) */ }
}

let locale = detectLocale();
// dict holds CONFIRMED translations (server value differs from the English
// key) and is persisted. sessionFallback holds keys the server had no
// translation for yet — kept in memory only, never persisted, so the next
// page load rechecks the server instead of being stuck on the old fallback.
let dict: Dict = loadCachedDict(locale);
let sessionFallback: Dict = {};
let version = 0;
const listeners = new Set<() => void>();
function notify() {
  version++;
  listeners.forEach((l) => l());
}

let pending: Dict = {};
let flushTimer: number | null = null;

function flush() {
  flushTimer = null;
  const batch = pending;
  pending = {};
  const keys = Object.keys(batch);
  if (keys.length === 0) return;
  const loc = locale;
  axios
    .post("/api/i18n/resolve", { locale: loc, strings: batch })
    .then((res) => {
      if (loc !== locale) return; // locale changed mid-flight; drop the stale batch
      const updates = res.data as Dict;
      let changed = false;
      for (const key of Object.keys(updates)) {
        const value = updates[key];
        if (value !== key) {
          dict[key] = value; // a real translation: safe to persist
          changed = true;
        } else {
          sessionFallback[key] = value; // still untranslated: retry next load
        }
      }
      if (changed) {
        dict = { ...dict };
        saveCachedDict(loc, dict);
      }
      notify();
    })
    .catch(() => { /* keep the English fallback already shown */ });
}

/** Resolves `text` (English, also the key) for the current locale. */
export function t(text: string): string {
  if (!text) return text;
  const known = dict[text] ?? sessionFallback[text];
  if (known !== undefined) return known;
  if (!(text in pending)) {
    pending[text] = text;
    if (flushTimer == null) flushTimer = window.setTimeout(flush, 150);
  }
  return text;
}

export function getLocale(): string {
  return locale;
}

export const locales = ["en", "ru"] as const;

export function setLocale(next: string) {
  if (next === locale) return;
  locale = next;
  try {
    localStorage.setItem(STORAGE_LOCALE_KEY, locale);
  } catch { /* ignore */ }
  dict = loadCachedDict(locale);
  sessionFallback = {};
  pending = {};
  notify();
}

export function subscribe(fn: () => void): () => void {
  listeners.add(fn);
  return () => listeners.delete(fn);
}

export function getVersion(): number {
  return version;
}
