import React from 'react';
import { FIELD_TYPES, MODES, isImmutableField } from './FieldsetProvider';
import TableEdit from './Table/Edit';
import TableView from './Table/View';
import BitmaskEdit from './Bitmask/Edit';
import BitmaskView from './Bitmask/View';
import AutocompleteEdit from './Autocomplete/Edit';

// Import all field components
import { 
  TextEdit, TextView, TextList,
  FloatEdit, FloatView, FloatList,
  DateEdit, DateView, DateList,
  SelectEdit, SelectView, SelectList,
  CheckboxEdit, CheckboxList,
  ImageEdit, ImageView, ImageList,
  MarkdownEdit, MarkdownView, MarkdownList,
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
    placeholder?: string;
    validation?: { [key: string]: any };
    options?: { [key: string]: any };
    readonly?: boolean;
    default_value?: any;
    linkModule?: string;
    unit?: string;
    zeroEmpty?: boolean;
    negativeClass?: string;
    positiveClass?: string;
    align?: string;
    columnWidth?: string;
    autocomplete?: string;
  };
  value?: any;
  onChange?: (value: any) => void;
  mode: number;
  disabled?: boolean;
  className?: string;
  module?: string; // owning module name (for fields that call module-scoped endpoints, e.g. Image)
  // All sibling form values, so dependent fields (e.g. a TABLE whose rows come
  // from another field's selected module) can react to them.
  formValues?: { [key: string]: any };
  // Optional label override. When provided (including ""), it replaces field.label
  // — the two-column form passes "" so the field renders no internal label and
  // the description column is the single source of the label.
  label?: string;
}

// Field type mapping
const FIELD_COMPONENTS = {
  [FIELD_TYPES.BITMASK_SELECT]: {
    [MODES.EDIT]: BitmaskEdit,
    [MODES.VIEW]: BitmaskView,
    [MODES.LIST]: BitmaskView,
  },
  [FIELD_TYPES.TABLE]: {
    [MODES.EDIT]: TableEdit,
    [MODES.VIEW]: TableView,
    [MODES.LIST]: TableView,
  },
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
  // DEPRECATED as a type: prefer TYPE_STRING + WithAutocomplete(...) (the
  // `autocomplete` option, handled above). These aliases keep any legacy
  // type:"Autocomplete" fields rendering as a String; the type-ahead widget is
  // enabled only when the `autocomplete` option is present.
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
    [MODES.EDIT]: CheckboxEdit,
    [MODES.VIEW]: CheckboxList,
    [MODES.LIST]: CheckboxList,
  },
  [FIELD_TYPES.CHECKBOX_SET]: {
    [MODES.EDIT]: CheckboxEdit,
    [MODES.VIEW]: CheckboxList,
    [MODES.LIST]: CheckboxList,
  },
  [FIELD_TYPES.MARKDOWN]: {
    [MODES.EDIT]: MarkdownEdit,
    [MODES.VIEW]: MarkdownView,
    [MODES.LIST]: MarkdownList,
  },
  [FIELD_TYPES.IMAGE]: {
    [MODES.EDIT]: ImageEdit,
    [MODES.VIEW]: ImageView,
    [MODES.LIST]: ImageList,
  },
};

// Default fallback component
const DefaultField: React.FC<BaseFieldProps> = ({ field, value, mode }) => (
  <div className="field-default">
    <label>{field.label}</label>
    <div className="field-content">
      {mode === MODES.EDIT || mode === MODES.CREATE ? (
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

  // Autocomplete is an OPTION on a String field, not a field type of its own:
  // `NewField("city", TYPE_STRING, ...).WithAutocomplete(...)` serializes an
  // `autocomplete` kind that renders the server-backed type-ahead input in
  // edit/create. Read it from the top-level flag or from options for robustness.
  const autocompleteKind = field.autocomplete ?? (field.options && (field.options as any).autocomplete);
  if (autocompleteKind && (mode === MODES.EDIT || mode === MODES.CREATE)) {
    return (
      <AutocompleteEdit
        id={field.name}
        fieldName={field.name}
        module={props.module}
        value={props.value}
        placeholder={field.placeholder || field.description || field.label}
        disabled={field.readonly || props.disabled}
        required={field.required}
        className={props.className}
        onChange={props.onChange}
        values={props.formValues}
      />
    );
  }
  
  // Find the appropriate component for the field type and mode. A field may
  // opt into the select widget (widget:"select") even when its stored type is
  // e.g. Int (a foreign-key id) — honour that so table-backed selects render as
  // dropdowns rather than raw number inputs.
  const effectiveType =
    field.options && field.options.widget === "select" && FIELD_COMPONENTS[FIELD_TYPES.SELECT]
      ? FIELD_TYPES.SELECT
      : field.type;
  const fieldComponents = FIELD_COMPONENTS[effectiveType];

  if (!fieldComponents) {
    console.warn(`No component found for field type: ${field.type}`);
    return <DefaultField {...props} />;
  }
  
  // Find component for specific mode, fallback to VIEW mode, then to default.
  // CREATE reuses the EDIT renderer (create is an editable form); without this,
  // CREATE would fall through to the read-only VIEW component.
  // Immutable identity/system fields (uuid, id, timestamps, created_by) are never
  // editable: render them with their VIEW component (read-only text) even in
  // edit/create, so they don't appear as inputs at all. `access` stays editable.
  const lookupMode = isImmutableField(field.name)
    ? MODES.VIEW
    : mode === MODES.CREATE
      ? MODES.EDIT
      : mode;
  let Component: React.ComponentType<any> = fieldComponents[lookupMode] ||
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
    label: props.label !== undefined ? props.label : field.label,
    // Explicit placeholder from the fieldset (field.placeholder) wins; falls
    // back to description/label. Used in edit/create and filter inputs.
    placeholder: field.placeholder || field.description || field.label,
    required: field.required,
    disabled: field.readonly || props.disabled,
    // Text-family widget: only multi-line types render a <textarea>; STRING and
    // autocomplete render a single-line <input>. Overridable via field.options.
    type:
      field.type === FIELD_TYPES.TEXT || field.type === FIELD_TYPES.HTML
        ? "text"
        : "varchar",
    // Pass field-specific options
    ...(field.options || {}),
  };

  const rendered = <Component {...enhancedProps} />;

  // Display modifiers apply to read-only rendering (list & view): zero-as-blank,
  // unit suffix, sign-based CSS class, and foreign-key links.
  if (mode === MODES.LIST || mode === MODES.VIEW) {
    return applyDisplayModifiers(field, props.value, rendered);
  }
  return rendered;
};

// applyDisplayModifiers wraps a rendered value with the field's formatting
// options (zeroEmpty, unit, sign class, linkModule).
function applyDisplayModifiers(
  field: any,
  value: any,
  rendered: React.ReactNode
): React.ReactElement {
  const num = typeof value === "number" ? value : parseFloat(value);
  const isNum = !isNaN(num);

  if (field.zeroEmpty && isNum && num === 0) {
    return <span className="text-muted">—</span>;
  }

  let className = "";
  if (isNum && num < 0 && field.negativeClass) className = field.negativeClass;
  if (isNum && num > 0 && field.positiveClass) className = field.positiveClass;

  let content: React.ReactNode = rendered;
  if (field.unit && value !== undefined && value !== null && value !== "") {
    content = (
      <>
        {rendered} <span className="field-unit text-muted">{field.unit}</span>
      </>
    );
  }
  if (field.linkModule && value) {
    content = <a href={`/${field.linkModule}/${value}/view`}>{content}</a>;
  }

  return <span className={className}>{content}</span>;
}

export default Field;