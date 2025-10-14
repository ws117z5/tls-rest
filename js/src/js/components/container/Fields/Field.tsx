import React from 'react';
import { FIELD_TYPES, MODES } from './FieldsetProvider';

// Import all field components
import { 
  TextEdit, TextView, TextList,
  FloatEdit, FloatView, FloatList,
  DateEdit, DateView, DateList,
  SelectEdit, SelectView, SelectList,
  CheckboxList,
  ImageEdit, ImageView, ImageList,
  ButtonView
} from './index';

// Base field component props
export interface BaseFieldProps {
  field: {
    name: string;
    type: string;
    label: string;
    required?: boolean;
    description?: string;
    validation?: { [key: string]: any };
    options?: { [key: string]: any };
    readonly?: boolean;
    default_value?: any;
  };
  value?: any;
  onChange?: (value: any) => void;
  mode: number;
  disabled?: boolean;
  className?: string;
}

// Field type mapping
const FIELD_COMPONENTS = {
  [FIELD_TYPES.STRING]: {
    [MODES.EDIT]: TextEdit,
    [MODES.VIEW]: TextView,
    [MODES.LIST]: TextList,
  },
  [FIELD_TYPES.TEXT]: {
    [MODES.EDIT]: TextEdit,
    [MODES.VIEW]: TextView,
    [MODES.LIST]: TextList,
  },
  [FIELD_TYPES.AUTOCOMPLETE]: {
    [MODES.EDIT]: TextEdit,
    [MODES.VIEW]: TextView,
    [MODES.LIST]: TextList,
  },
  [FIELD_TYPES.AUTOCOMPLETE_TEXT]: {
    [MODES.EDIT]: TextEdit,
    [MODES.VIEW]: TextView,
    [MODES.LIST]: TextList,
  },
  [FIELD_TYPES.INT]: {
    [MODES.EDIT]: FloatEdit,
    [MODES.VIEW]: FloatView,
    [MODES.LIST]: FloatList,
  },
  [FIELD_TYPES.FLOAT]: {
    [MODES.EDIT]: FloatEdit,
    [MODES.VIEW]: FloatView,
    [MODES.LIST]: FloatList,
  },
  [FIELD_TYPES.MONEY]: {
    [MODES.EDIT]: FloatEdit,
    [MODES.VIEW]: FloatView,
    [MODES.LIST]: FloatList,
  },
  [FIELD_TYPES.DATE]: {
    [MODES.EDIT]: DateEdit,
    [MODES.VIEW]: DateView,
    [MODES.LIST]: DateList,
  },
  [FIELD_TYPES.DATE_TIME]: {
    [MODES.EDIT]: DateEdit,
    [MODES.VIEW]: DateView,
    [MODES.LIST]: DateList,
  },
  [FIELD_TYPES.SELECT]: {
    [MODES.EDIT]: SelectEdit,
    [MODES.VIEW]: SelectView,
    [MODES.LIST]: SelectList,
  },
  [FIELD_TYPES.SELECT_ADDNEW]: {
    [MODES.EDIT]: SelectEdit,
    [MODES.VIEW]: SelectView,
    [MODES.LIST]: SelectList,
  },
  [FIELD_TYPES.CHECKBOX]: {
    [MODES.EDIT]: CheckboxList,
    [MODES.VIEW]: CheckboxList,
    [MODES.LIST]: CheckboxList,
  },
  [FIELD_TYPES.CHECKBOX_SET]: {
    [MODES.EDIT]: CheckboxList,
    [MODES.VIEW]: CheckboxList,
    [MODES.LIST]: CheckboxList,
  },
};

// Default fallback component
const DefaultField: React.FC<BaseFieldProps> = ({ field, value, mode }) => (
  <div className="field-default">
    <label>{field.label}</label>
    <div className="field-content">
      {mode === MODES.EDIT ? (
        <input 
          type="text" 
          value={value || ''} 
          className="form-control"
          placeholder={field.label}
        />
      ) : (
        <span>{value || '-'}</span>
      )}
    </div>
  </div>
);

// Main Field component
export const Field: React.FC<BaseFieldProps> = (props) => {
  const { field, mode } = props;
  
  // Find the appropriate component for the field type and mode
  const fieldComponents = FIELD_COMPONENTS[field.type];
  
  if (!fieldComponents) {
    console.warn(`No component found for field type: ${field.type}`);
    return <DefaultField {...props} />;
  }
  
  // Find component for specific mode, fallback to VIEW mode, then to default
  let Component: React.ComponentType<any> = fieldComponents[mode] || 
                   fieldComponents[MODES.VIEW] || 
                   fieldComponents[MODES.EDIT] ||
                   DefaultField;

  if (!Component) {
    console.warn(`No component found for field type: ${field.type} and mode: ${mode}`);
    Component = DefaultField;
  }

  // Enhanced props for the field component
  const enhancedProps = {
    ...props,
    id: field.name,
    label: field.label,
    placeholder: field.description || field.label,
    required: field.required,
    disabled: field.readonly || props.disabled,
    // Pass field-specific options
    ...(field.options || {}),
  };

  return <Component {...enhancedProps} />;
};

export default Field;