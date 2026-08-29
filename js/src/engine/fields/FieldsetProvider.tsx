import React, { createContext, useContext, useEffect, useState } from 'react';
import axios from 'axios';
import Auth from '@controllers/auth';

// System/default fields that are managed by the backend. They are never
// editable (except access, which is admin-editable), and are only shown in
// list/view/edit to administrators. The backend is authoritative — it filters
// these from schema and data — this set keeps the client in agreement.
export const SYSTEM_FIELDS = new Set([
  'id', 'uuid', 'created', 'updated', 'created_by', 'access',
]);

// Identity/system fields are managed by the backend and must never be edited by
// the user — with the sole exception of `access`, which admins may change. The
// form uses this to force these inputs read-only regardless of the fieldset.
export const isImmutableField = (name: string): boolean =>
  SYSTEM_FIELDS.has(name) && name !== 'access';

// Field type constants matching backend
export const FIELD_TYPES = {
  HTML: 'Html',
  CHECKBOX: 'Checkbox',
  CHECKBOX_SET: 'CheckboxSet', 
  CHECKBOX_AJAX: 'CheckboxAjax',
  STRING: 'String',
  AUTOCOMPLETE: 'Autocomplete',
  TEXT: 'Text',
  JSON: 'Json',
  TABLE: 'Table',
  BITMASK_SELECT: 'BitmaskSelect',
  AUTOCOMPLETE_TEXT: 'AutocompleteText',
  DATE: 'Date',
  DATE_TIME: 'DateTime',
  COLOR: 'Color',
  WEEK: 'Week',
  INT: 'Int',
  FLOAT: 'Float',
  MONEY: 'Money',
  MONEY_WITH_CURRENCY: 'MoneyWithCurrency',
  SELECT: 'Select',
  SELECT_ADDNEW: 'SelectAddNew',
  SELECT_CHILD: 'SelectChild',
  SELECT2_MULTIPLE: 'Select2Multiple',
  ACTIVE_INACTIVE: 'ActiveInactive',
  YES_NO: 'YesNo',
  MONTH: 'Month',
  MARKDOWN: 'Markdown',
  IMAGE: 'Image'
};

// Mode constants
export const MODES = {
  LIST: 0b000001,
  VIEW: 0b000010,
  EDIT: 0b000100,
  LOG: 0b001000,
  MULTIPLEUPDATE: 0b010000,
  SUBMIT: 0b100000,
  CREATE: 0b1000000,
  DELETE: 0b10000000,
  READONLY: 0b000011,
  EDITSUBMIT: 0b100100,
  ALL: 0b11111111
};

// Types
interface Field {
  name: string;
  type: string;
  required: boolean;
  label: string;
  description?: string;
  mode?: number;
  filterable?: boolean;
  sortable?: boolean;
  searchable?: boolean;
  virtual?: boolean;
  readonly?: boolean;
  default_value?: any;
  validation?: { [key: string]: any };
  options?: { [key: string]: any };
}

interface FieldsetData {
  fields: Field[];
  id: string;
  name: string;
}

interface FieldsetContextType {
  fieldset: FieldsetData | null;
  loading: boolean;
  error: string | null;
  mode: number;
  module: string;
  refetchFieldset: () => void;
  isFieldVisible: (field: Field) => boolean;
  getFieldsForMode: () => Field[];
}

// Fieldset Context
const FieldsetContext = createContext<FieldsetContextType | undefined>(undefined);

export const useFieldset = () => {
  const context = useContext(FieldsetContext);
  if (!context) {
    throw new Error('useFieldset must be used within a FieldsetProvider');
  }
  return context;
};

// Fieldset Provider Component
interface FieldsetProviderProps {
  children: React.ReactNode;
  module: string;
  mode?: number;
  // Optional inline fieldset. When provided (e.g. by a FieldsetPage that already
  // fetched {Data, Fieldset} from a single endpoint), it is used directly and no
  // /api/modules/{module}/fieldset request is made.
  fieldset?: FieldsetData | null;
}

export const FieldsetProvider: React.FC<FieldsetProviderProps> = ({ children, module, mode = MODES.ALL, fieldset: inlineFieldset = null }) => {
  const [fieldset, setFieldset] = useState<FieldsetData | null>(inlineFieldset);
  const [loading, setLoading] = useState(!inlineFieldset);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (inlineFieldset) {
      setFieldset(inlineFieldset);
      setLoading(false);
      return;
    }
    if (module) {
      fetchFieldset();
    }
  }, [module, inlineFieldset]);

  const fetchFieldset = async () => {
    // One cached fieldset per module (mode is filtered client-side, not fetched).
    const cacheKey = `fieldset:${module}`;
    let cached: { hash?: string; data?: FieldsetData } | null = null;
    try {
      const raw = localStorage.getItem(cacheKey);
      if (raw) cached = JSON.parse(raw);
    } catch {
      cached = null;
    }

    try {
      setLoading(true);
      setError(null);

      // POST; hash (if any) rides as a query param so the server can answer 304.
      const url = `/api/modules/${module}/fieldset`;
      const response = await axios.post(url, 
        cached?.hash ? { hash: cached.hash } : {}, // POST body
        { validateStatus: (s: number) => s === 304 || (s >= 200 && s < 300), }
      );

      if (response.status === 304) {
        if (cached?.data) {
          setFieldset(cached.data);
        } else {
          // Cold cache but server said 304 (shouldn't normally happen) — refetch
          // without the hash to get the full fieldset.
          const full = await axios.post(url, null);
          setFieldset(full.data);
          try {
            localStorage.setItem(cacheKey, JSON.stringify({ hash: full.data?.hash, data: full.data }));
          } catch {}
        }
      } else {
        setFieldset(response.data);
        try {
          localStorage.setItem(cacheKey, JSON.stringify({ hash: response.data?.hash, data: response.data }));
        } catch {}
      }
    } catch (err: unknown) {
      if (err instanceof Error) {
        console.error('Failed to fetch fieldset:', err);
        setError(err.message);
      } else {
        console.error('An unexpected error occurred:', err);
      }
    } finally {
      setLoading(false);
    }
  };

  const contextValue = {
    fieldset,
    loading,
    error,
    mode,
    module,
    refetchFieldset: fetchFieldset,
    // Helper function to check if field should be visible in current mode
    isFieldVisible: (field: Field) => {
      if (!field || !field.mode) return true;
      return (field.mode & mode) !== 0;
    },
    // Helper function to get fields for current mode
    getFieldsForMode: () => {
      if (!fieldset || !fieldset.fields) return [];
      const admin = Auth.isAdmin();
      let filtered = fieldset.fields.filter(field => {
          let modeCheck = (!field.mode || (field.mode & mode) !== 0)
          let adminCheck = (!SYSTEM_FIELDS.has(field.name) || admin)
          return modeCheck && adminCheck
        }
      );
      return filtered
    }
  };

  return (
    <FieldsetContext.Provider value={contextValue}>
      {children}
    </FieldsetContext.Provider>
  );
};

export default FieldsetProvider;