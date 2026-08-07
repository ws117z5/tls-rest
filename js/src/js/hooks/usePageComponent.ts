// usePageComponent.ts
import { useState, useEffect, useRef } from "react";
import axios from "axios";
import Config from "../controllers/config";
import Functional from "../controllers/functional";

export interface PageComponentState<T = any> {
    Name: string;
    Data: T[];
    Fieldset: { [key: string]: any };
    loading: boolean;
    error?: any;
}

export interface PageComponentOptions {
    name: string;
    href?: string;
}

export function usePageComponent<T = any>({ name, href }: PageComponentOptions = { name: ""}) {
  // stable id for the lifetime of the component (replaces this.uuid)
  const uuidRef = useRef<string>(Functional.guid());

  const [state, setState] = useState<PageComponentState<T>>({
    Name: name,
    Data: [],
    Fieldset: {},
    loading: true,
  });

  useEffect(() => {
    if (!href) {
      setState((s) => ({ ...s, loading: false }));
      return;
    }

    let cancelled = false;

    (async () => {
      try {
        const response = await axios.get(Config.serverURL + href);
        if (cancelled) return;

        setState((s) => ({
          ...s,
          loading: false,
          Data: response.data?.Data ?? s.Data,
          Fieldset: response.data?.Fieldset ?? s.Fieldset,
        }));
      } catch (error) {
        if (cancelled) return;
        console.error(`Error loading API for ${href}:`, error);
        setState((s) => ({ ...s, loading: false, error }));
      }
    })();

    return () => {
      cancelled = true; // avoid setState after unmount
    };
  }, [href]);



  return {
    uuid: uuidRef.current,
    Data: state.Data,
    Fieldset: state.Fieldset,
    loading: state.loading,
    error: state.error,
    setState,
  };
}