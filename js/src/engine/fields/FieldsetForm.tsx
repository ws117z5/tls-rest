import React, { Component } from 'react';
import { Field, BaseFieldProps } from './Field';
import { useFieldset, MODES } from './FieldsetProvider';

// Form data interface
interface FormData {
  [key: string]: any;
}

// FieldsetForm props
interface FieldsetFormProps {
  mode?: number;
  data?: FormData;
  onSubmit?: (data: FormData) => void;
  onChange?: (field: string, value: any) => void;
  onValidation?: (errors: { [key: string]: string }) => void;
  className?: string;
  disabled?: boolean;
  showRequiredIndicator?: boolean;
}

interface FieldsetFormState {
  formData: FormData;
  errors: { [key: string]: string };
  touched: { [key: string]: boolean };
}

// Hook-based wrapper for class component
const FieldsetFormWrapper: React.FC<FieldsetFormProps> = (props) => {
  const fieldsetContext = useFieldset();
  return <FieldsetFormClass {...props} fieldsetContext={fieldsetContext} />;
};

// Class component with fieldset context
interface FieldsetFormClassProps extends FieldsetFormProps {
  fieldsetContext: any;
}

class FieldsetFormClass extends Component<FieldsetFormClassProps, FieldsetFormState> {
  static defaultProps = {
    mode: MODES.EDIT,
    data: {},
    className: '',
    disabled: false,
    showRequiredIndicator: true,
  };

  constructor(props: FieldsetFormClassProps) {
    super(props);
    this.state = {
      formData: { ...props.data },
      errors: {},
      touched: {},
    };
  }

  componentDidUpdate(prevProps: FieldsetFormClassProps) {
    if (prevProps.data !== this.props.data) {
      this.setState({ 
        formData: { ...this.props.data },
        errors: {},
        touched: {}
      });
    }
  }

  validateField = (field: any, value: any): string => {
    const { validation = {}, required, type } = field;
    
    // Required validation
    if (required && (value === null || value === undefined || value === '')) {
      return `${field.label} is required`;
    }

    // Type-specific validation
    if (value !== null && value !== undefined && value !== '') {
      // String length validation
      if (type === 'String' || type === 'Text') {
        if (validation.minLength && value.length < validation.minLength) {
          return `${field.label} must be at least ${validation.minLength} characters`;
        }
        if (validation.maxLength && value.length > validation.maxLength) {
          return `${field.label} must not exceed ${validation.maxLength} characters`;
        }
      }

      // Numeric validation
      if (type === 'Int' || type === 'Float') {
        const numValue = parseFloat(value);
        if (isNaN(numValue)) {
          return `${field.label} must be a valid number`;
        }
        if (validation.min !== undefined && numValue < validation.min) {
          return `${field.label} must be at least ${validation.min}`;
        }
        if (validation.max !== undefined && numValue > validation.max) {
          return `${field.label} must not exceed ${validation.max}`;
        }
      }

      // Email validation
      if (validation.email && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value)) {
        return `${field.label} must be a valid email address`;
      }

      // Pattern validation
      if (validation.pattern) {
        const regex = new RegExp(validation.pattern);
        if (!regex.test(value)) {
          return validation.patternMessage || `${field.label} format is invalid`;
        }
      }
    }

    return '';
  };

  validateForm = (): boolean => {
    const { fieldsetContext, mode } = this.props;
    const { formData } = this.state;
    
    if (!fieldsetContext.fieldset) return true;
    
    const fields = fieldsetContext.getFieldsForMode();
    const newErrors: { [key: string]: string } = {};
    let isValid = true;

    fields.forEach((field: any) => {
      if (!field.readonly && !field.virtual) {
        const error = this.validateField(field, formData[field.name]);
        if (error) {
          newErrors[field.name] = error;
          isValid = false;
        }
      }
    });

    this.setState({ errors: newErrors });
    
    if (this.props.onValidation) {
      this.props.onValidation(newErrors);
    }

    return isValid;
  };

  handleFieldChange = (fieldName: string, value: any) => {
    const newFormData = {
      ...this.state.formData,
      [fieldName]: value
    };

    this.setState({
      formData: newFormData,
      touched: {
        ...this.state.touched,
        [fieldName]: true
      }
    });

    if (this.props.onChange) {
      this.props.onChange(fieldName, value);
    }

    // Validate field on change if it's been touched
    if (this.state.touched[fieldName]) {
      const field = this.props.fieldsetContext.fieldset?.fields.find((f: any) => f.name === fieldName);
      if (field) {
        const error = this.validateField(field, value);
        this.setState({
          errors: {
            ...this.state.errors,
            [fieldName]: error
          }
        });
      }
    }
  };

  handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    
    if (this.validateForm() && this.props.onSubmit) {
      this.props.onSubmit(this.state.formData);
    }
  };

  // Standard column widths (px) used when the fieldset doesn't predefine one.
  static STD_DESC_WIDTH = 220;

  // Compute the shared column widths for the whole form. If any field predefines
  // a description/value width, the column takes the widest of those; otherwise a
  // standard width is used (value defaults to flex-fill when unset).
  computeLayout = (fields: any[]) => {
    const descWidths = fields
      .filter((f: any) => !(f.options && f.options.hideDescription))
      .map((f: any) => Number(f.options && f.options.descriptionWidth))
      .filter((n: number) => !isNaN(n) && n > 0);
    const valueWidths = fields
      .map((f: any) => Number(f.options && f.options.valueWidth))
      .filter((n: number) => !isNaN(n) && n > 0);

    return {
      descWidth: descWidths.length ? Math.max(...descWidths) : FieldsetFormClass.STD_DESC_WIDTH,
      valueWidth: valueWidths.length ? Math.max(...valueWidths) : null, // null => flex fill
    };
  };

  renderField = (field: any, layout: { descWidth: number; valueWidth: number | null }) => {
    const { mode, disabled } = this.props;
    const { formData, errors, touched } = this.state;

    const hideDescription = !!(field.options && field.options.hideDescription);

    const fieldProps: BaseFieldProps = {
      field,
      value: formData[field.name] || field.default_value,
      onChange: (value) => this.handleFieldChange(field.name, value),
      mode: mode || MODES.EDIT,
      disabled: disabled || field.readonly,
      module: this.props.fieldsetContext?.module,
      className: touched[field.name] && errors[field.name] ? 'is-invalid' : '',
      // The description column owns the label, so suppress the field's own.
      label: '',
    };

    const valueStyle: React.CSSProperties = layout.valueWidth
      ? { flex: `0 0 ${layout.valueWidth}px`, maxWidth: layout.valueWidth, minWidth: 0 }
      : { flex: '1 1 auto', minWidth: 0 };

    const valueCell = (
      <div className="fieldset-cell fieldset-cell-value" style={valueStyle}>
        <Field {...fieldProps} />
        {touched[field.name] && errors[field.name] && (
          <div className="invalid-feedback d-block">{errors[field.name]}</div>
        )}
      </div>
    );

    // Field opted out of the description column: value spans the full row.
    if (hideDescription) {
      return (
        <div key={field.name} className="fieldset-row d-flex mb-3">
          {valueCell}
        </div>
      );
    }

    return (
      <div key={field.name} className="fieldset-row d-flex gap-3 mb-3 align-items-start">
        <div
          className="fieldset-cell fieldset-cell-desc"
          style={{ flex: `0 0 ${layout.descWidth}px`, maxWidth: layout.descWidth }}
        >
          <div className="fw-semibold">
            {field.label}
            {field.required && <span className="text-danger"> *</span>}
          </div>
          {field.description && (
            <div className="text-muted small">{field.description}</div>
          )}
        </div>
        {valueCell}
      </div>
    );
  };

  render() {
    const { 
      fieldsetContext, 
      mode, 
      className, 
      showRequiredIndicator 
    } = this.props;

    if (fieldsetContext.loading) {
      return (
        <div className="d-flex justify-content-center p-4">
          <div className="spinner-border" role="status">
            <span className="sr-only">Loading fieldset...</span>
          </div>
        </div>
      );
    }

    if (fieldsetContext.error) {
      return (
        <div className="alert alert-danger">
          Failed to load form configuration: {fieldsetContext.error}
        </div>
      );
    }

    if (!fieldsetContext.fieldset) {
      return (
        <div className="alert alert-warning">
          No fieldset configuration available
        </div>
      );
    }

    const fields = fieldsetContext.getFieldsForMode();
    const isEditMode = mode === MODES.EDIT || mode === MODES.EDITSUBMIT || mode === MODES.CREATE;
    const layout = this.computeLayout(fields);

    return (
      <form onSubmit={this.handleSubmit} className={`fieldset-form ${className}`}>
        {showRequiredIndicator && isEditMode && (
          <div className="mb-3 text-muted small">
            <span className="text-danger">*</span> Required fields
          </div>
        )}

        {fields.map((field: any) => this.renderField(field, layout))}
        
        {isEditMode && this.props.onSubmit && (
          <div className="form-actions mt-4">
            <button type="submit" className="btn btn-primary">
              Save
            </button>
            <button type="button" className="btn btn-secondary ms-2">
              Cancel
            </button>
          </div>
        )}
      </form>
    );
  }
}

export default FieldsetFormWrapper;