import React, { createContext, useContext, useEffect, useState } from 'react';
import axios from 'axios';

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
  MONTH: 'Month'
};

// Mode constants
export const MODES = {
  LIST: 0b000001,
  VIEW: 0b000010,
  EDIT: 0b000100,
  LOG: 0b001000,
  MULTIPLEUPDATE: 0b010000,
  SUBMIT: 0b100000,
  READONLY: 0b000011,
  EDITSUBMIT: 0b100100,
  ALL: 0b111111
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
}

export const FieldsetProvider: React.FC<FieldsetProviderProps> = ({ children, module, mode = MODES.ALL }) => {
  const [fieldset, setFieldset] = useState<FieldsetData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  useEffect(() => {
    if (module) {
      fetchFieldset();
    }
  }, [module, mode]);

  const fetchFieldset = async () => {
    try {
      setLoading(true);
      setError(null);
      
      // Request fieldset configuration from backend
      const response = await axios.get(`/api/modules/${module}/fieldset`, {
        params: { mode }
      });
      
      setFieldset(response.data);
    } catch (err) {
      console.error('Failed to fetch fieldset:', err);
      setError(err.message);
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
    isFieldVisible: (field) => {
      if (!field || !field.mode) return true;
      return (field.mode & mode) !== 0;
    },
    // Helper function to get fields for current mode
    getFieldsForMode: () => {
      if (!fieldset || !fieldset.fields) return [];
      return fieldset.fields.filter(field => 
        !field.mode || (field.mode & mode) !== 0
      );
    }
  };

  return (
    <FieldsetContext.Provider value={contextValue}>
      {children}
    </FieldsetContext.Provider>
  );
};

export default FieldsetProvider;