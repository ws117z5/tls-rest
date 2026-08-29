import React, { createContext, useContext, useMemo } from "react";
import { Field as EngineField } from "./Field";
import { useFieldset, isImmutableField } from "./FieldsetProvider";

interface FormLayoutBridgeProps {
  formData: any;
  setRecord: React.Dispatch<React.SetStateAction<any>>;
  mode: number;
  module: string;
  errors?: Record<string, string>;
  touched?: Record<string, boolean>;
  children: React.ReactNode;
}

export const FormLayoutBridge: React.FC<FormLayoutBridgeProps> = ({
  formData,
  setRecord,
  mode,
  module,
  errors = {},
  touched = {},
  children,
}) => {
  const { getFieldsForMode, loading } = useFieldset();

  const contextValue = useMemo<FormLayoutContextValue>(() => {
    // 1. Convert getFieldsForMode() array to a Record<fieldName, FieldMeta>
    const activeFields = getFieldsForMode();
    const fieldsMap: Record<string, any> = {};
    activeFields.forEach((field) => {
      fieldsMap[field.name] = field;
    });

    // 2. Build the exact context value required by <Field name="..." />
    return {
      fields: fieldsMap,
      formData: formData || {},
      mode,
      module,
      errors,
      touched,
      onFieldChange: (name: string, value: any) => {
        setRecord((prev: any) => ({
          ...prev,
          [name]: value,
        }));
      },
      // Accessors for custom containers to inspect the live form.
      getValue: (name: string) => (formData || {})[name],
      getField: (name: string) => fieldsMap[name],
      getOptions: (name: string) => (fieldsMap[name] && fieldsMap[name].options) || {},
    };
  }, [getFieldsForMode, formData, mode, module, errors, touched, setRecord]);

  if (loading) return null;

  return (
    <FormLayoutContext.Provider value={contextValue}>
      {children}
    </FormLayoutContext.Provider>
  );
};

// FormLayoutContext exposes the surrounding fieldset form to a custom layout,
// so a placed <Field name="X" /> renders exactly like the default form would.
export interface FormLayoutContextValue {
  fields: Record<string, any>; // fieldName -> field meta
  formData: Record<string, any>;
  mode: number;
  disabled?: boolean;
  module?: string;
  errors: Record<string, string>;
  touched: Record<string, boolean>;
  onFieldChange: (name: string, value: any) => void;
  // Accessors for custom View/Edit containers.
  getValue: (name: string) => any; // current value of a field
  getField: (name: string) => any; // field meta (type, options, validation, ...)
  getOptions: (name: string) => Record<string, any>; // field.options ({} if none)
}

export const FormLayoutContext = createContext<FormLayoutContextValue | null>(null);

interface LayoutFieldProps {
  name: string;
  label?: string; // optional label override ("" hides the internal label)
  cssClass?: string;
}

// Field renders a single named field from the surrounding fieldset form. Use it
// inside a custom module layout (js/src/modules/<module>/Edit.tsx):
//
//   import { Field } from "@engine/fields/FormLayout";
//   export default () => (<div className="grid"><Field name="title" /><Field name="images" /></div>);
export const Field: React.FC<LayoutFieldProps> = ({ name, label, cssClass }) => {
  const ctx = useContext(FormLayoutContext);
  if (!ctx) return null;
  const meta = ctx.fields[name];
  if (!meta) return null;
  const invalid = ctx.touched[name] && ctx.errors[name];
  const css = invalid ? "is-invalid" : (cssClass ? cssClass : "");
  // uuid and other identity/system fields are never editable, whatever the
  // fieldset says (access is the one admins may edit).
  const forcedReadonly = meta.readonly || isImmutableField(name);
  return (
    <EngineField
      field={meta}
      value={ctx.formData[name]}
      onChange={(v: any) => ctx.onFieldChange(name, v)}
      mode={ctx.mode}
      disabled={ctx.disabled || forcedReadonly}
      module={ctx.module}
      formValues={ctx.formData}
      label={label}
      className={css}
    />
  );
};

// useFormLayout lets a custom layout read/inspect the form (e.g. to show a value
// conditionally). Returns null outside a layout.
export function useFormLayout(): FormLayoutContextValue | null {
  return useContext(FormLayoutContext);
}

// WithLayout renders a custom module container (modules/<m>/View.tsx, Edit.tsx)
// and passes it — on top of `extra` (module, mode, record, submit, ...) — the
// field-access params so the container can check values and options:
//
//   fields   : Record<name, meta>        values  : Record<name, value>
//   getValue(name)  -> current value     getField(name)  -> field meta
//   getOptions(name) -> field.options
//
// Must be rendered inside <FormLayoutBridge>.
interface WithLayoutProps {
  component: React.ComponentType<any>;
  extra?: Record<string, any>;
}
export const WithLayout: React.FC<WithLayoutProps> = ({ component: C, extra = {} }) => {
  const layout = useContext(FormLayoutContext);
  return (
    <C
      {...extra}
      fields={layout?.fields ?? {}}
      values={layout?.formData ?? {}}
      errors={layout?.errors ?? {}}
      touched={layout?.touched ?? {}}
      getValue={(name: string) => layout?.formData?.[name]}
      getField={(name: string) => layout?.fields?.[name]}
      getOptions={(name: string) => layout?.fields?.[name]?.options ?? {}}
    />
  );
};