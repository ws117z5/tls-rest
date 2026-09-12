import { useSyncExternalStore } from "react";
import { getVersion, subscribe, t } from "./i18n";

/** Re-renders the component whenever the locale or a translation resolves. */
export default function useT() {
  useSyncExternalStore(subscribe, getVersion);
  return t;
}
