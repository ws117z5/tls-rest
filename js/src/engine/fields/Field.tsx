import React from 'react';
import { FIELD_TYPES, MODES, isImmutableField } from './FieldsetProvider';
import { t } from '@engine/i18n';

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

// --- Field renderer discovery --------------------------------------------
//
// Field renderers are picked up from the filesystem at build time, the same way
// per-module list/view/edit overrides are (see controllers/registry.ts): every
// fields/<Name>/{Edit,View,List}.tsx is bundled automatically and keyed by its
// directory name. A new field type is just a new directory — nothing to wire up
// here. TYPE_DIR below only lists the handful of types whose renderer lives in a
// differently-named directory.

type ModeMap = Partial<Record<number, React.ComponentType<any>>>;

const wpMeta = import.meta as unknown as {
  webpackContext: (
    request: string,
    options: { recursive: boolean; regExp: RegExp }
  ) => __WebpackModuleApi.RequireContext;
};

const FILE_MODE: Record<string, number> = {
  Edit: MODES.EDIT,
  View: MODES.VIEW,
  List: MODES.LIST,
};

const COMPONENT_KEY = /^\.\/([A-Za-z0-9]+)\/(Edit|View|List)\.tsx$/;

// directory name -> { [mode]: Component }
const COMPONENTS: Record<string, ModeMap> = {};

const fieldCtx = (import.meta as any).webpackContext('.', { recursive: true, regExp: /^\.\/([A-Za-z0-9]+)\/(Edit|View|List)\.tsx$/ });
fieldCtx.keys().forEach((key: string) => {
  const m = COMPONENT_KEY.exec(key);
  if (!m) return;
  const mod = fieldCtx(key) as any;
  const component = mod && (mod.default || mod);
  if (!component) return;
  if (!COMPONENTS[m[1]]) COMPONENTS[m[1]] = {};
  COMPONENTS[m[1]][FILE_MODE[m[2]]] = component;
});

// Historical quirk: a checkbox in view mode is rendered with its list widget
// (a disabled checkbox), not its own badge-style View component.
if (COMPONENTS.Checkbox && COMPONENTS.Checkbox[MODES.LIST]) {
  COMPONENTS.Checkbox[MODES.VIEW] = COMPONENTS.Checkbox[MODES.LIST];
}

// Field types whose renderer directory is not simply their type string. Any type
// not listed uses the directory named exactly after it (Text, Float, Date,
// Select, Checkbox, Markdown, Image, Table, Password, TimeDuration, ...).
const TYPE_DIR: Record<string, string> = {
  [FIELD_TYPES.STRING]: 'Text',
  [FIELD_TYPES.AUTOCOMPLETE]: 'Text',
  [FIELD_TYPES.AUTOCOMPLETE_TEXT]: 'Text',
  [FIELD_TYPES.INT]: 'Float',
  [FIELD_TYPES.MONEY]: 'Float',
  [FIELD_TYPES.DATE_TIME]: 'Date',
  [FIELD_TYPES.SELECT_ADDNEW]: 'Select',
  [FIELD_TYPES.CHECKBOX_SET]: 'Checkbox',
  [FIELD_TYPES.BITMASK_SELECT]: 'Bitmask',
};

// componentsForType returns the { [mode]: Component } triple for a field type,
// or undefined when no renderer directory matches.
function componentsForType(type: string): ModeMap | undefined {
  const found = COMPONENTS[TYPE_DIR[type] || type];
  return found && Object.keys(found).length ? found : undefined;
}

const AutocompleteEdit = COMPONENTS['Autocomplete']?.[MODES.EDIT] as
  | React.ComponentType<any>
  | undefined;

// Default fallback component
const DefaultField: React.FC<BaseFieldProps> = ({ field, value, mode }) => (
  <div className="field-default">
    <label>{field.label ? t(field.label) : field.label}</label>
    <div className="field-content">
      {mode === MODES.EDIT || mode === MODES.CREATE ? (
        <input
          type="text"
          value={value || ''}
          className="form-control"
          placeholder={field.label ? t(field.label) : field.label}
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
  if (autocompleteKind && AutocompleteEdit && (mode === MODES.EDIT || mode === MODES.CREATE)) {
    return (
      <AutocompleteEdit
        id={field.name}
        fieldName={field.name}
        module={props.module}
        value={props.value}
        placeholder={t(field.placeholder || field.description || field.label || '')}
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
    field.options && field.options.widget === "select" && componentsForType(FIELD_TYPES.SELECT)
      ? FIELD_TYPES.SELECT
      : field.type;
  const fieldComponents = componentsForType(effectiveType);

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
    label: props.label !== undefined ? props.label : (field.label ? t(field.label) : field.label),
    // Explicit placeholder from the fieldset (field.placeholder) wins; falls
    // back to description/label. Used in edit/create and filter inputs.
    placeholder: t(field.placeholder || field.description || field.label || ''),
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
        {rendered} <span className="field-unit text-muted">{t(field.unit)}</span>
      </>
    );
  }
  if (field.linkModule && value) {
    content = <a href={`/${field.linkModule}/${value}/view`}>{content}</a>;
  }

  return <span className={className}>{content}</span>;
}

export default Field;