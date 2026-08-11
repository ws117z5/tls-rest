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

  renderField = (field: any) => {
    const { mode, disabled } = this.props;
    const { formData, errors, touched } = this.state;
    
    const fieldProps: BaseFieldProps = {
      field,
      value: formData[field.name] || field.default_value,
      onChange: (value) => this.handleFieldChange(field.name, value),
      mode: mode || MODES.EDIT,
      disabled: disabled || field.readonly,
      module: this.props.fieldsetContext?.module,
      className: touched[field.name] && errors[field.name] ? 'is-invalid' : ''
    };

    return (
      <div key={field.name} className="mb-3">
        <Field {...fieldProps} />
        {touched[field.name] && errors[field.name] && (
          <div className="invalid-feedback d-block">
            {errors[field.name]}
          </div>
        )}
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

    return (
      <form onSubmit={this.handleSubmit} className={`fieldset-form ${className}`}>
        {showRequiredIndicator && isEditMode && (
          <div className="mb-3 text-muted small">
            <span className="text-danger">*</span> Required fields
          </div>
        )}
        
        {fields.map(this.renderField)}
        
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