import React, { Component } from 'react';
import { Field } from './Field';
import { useFieldset, MODES } from './FieldsetProvider';

// List data interface
interface ListData {
  [key: string]: any;
}

// FieldsetList props
interface FieldsetListProps {
  data?: ListData[];
  onRowClick?: (row: ListData, index: number) => void;
  onEdit?: (row: ListData, index: number) => void;
  onDelete?: (row: ListData, index: number) => void;
  onView?: (row: ListData, index: number) => void;
  className?: string;
  showActions?: boolean;
  sortable?: boolean;
  filterable?: boolean;
  pagination?: {
    page: number;
    limit: number;
    total: number;
    onPageChange: (page: number) => void;
  };
}

interface FieldsetListState {
  sortField?: string;
  sortDirection?: 'asc' | 'desc';
  filters: { [key: string]: any };
}

// Hook-based wrapper
const FieldsetListWrapper: React.FC<FieldsetListProps> = (props) => {
  const fieldsetContext = useFieldset();
  return <FieldsetListClass {...props} fieldsetContext={fieldsetContext} />;
};

// Class component
interface FieldsetListClassProps extends FieldsetListProps {
  fieldsetContext: any;
}

class FieldsetListClass extends Component<FieldsetListClassProps, FieldsetListState> {
  static defaultProps = {
    data: [],
    className: '',
    showActions: true,
    sortable: true,
    filterable: false,
  };

  constructor(props: FieldsetListClassProps) {
    super(props);
    this.state = {
      sortField: undefined,
      sortDirection: 'asc',
      filters: {},
    };
  }

  handleSort = (fieldName: string) => {
    const { sortable } = this.props;
    if (!sortable) return;

    const { sortField, sortDirection } = this.state;
    
    let newDirection: 'asc' | 'desc' = 'asc';
    if (sortField === fieldName && sortDirection === 'asc') {
      newDirection = 'desc';
    }

    this.setState({
      sortField: fieldName,
      sortDirection: newDirection,
    });
  };

  handleFilter = (fieldName: string, value: any) => {
    this.setState({
      filters: {
        ...this.state.filters,
        [fieldName]: value,
      },
    });
  };

  getSortedData = (): ListData[] => {
    const { data } = this.props;
    const { sortField, sortDirection } = this.state;
    
    if (!sortField || !data) return data || [];

    return [...data].sort((a, b) => {
      const aVal = a[sortField];
      const bVal = b[sortField];
      
      if (aVal < bVal) return sortDirection === 'asc' ? -1 : 1;
      if (aVal > bVal) return sortDirection === 'asc' ? 1 : -1;
      return 0;
    });
  };

  getFilteredData = (): ListData[] => {
    const sortedData = this.getSortedData();
    const { filters } = this.state;
    
    const filterKeys = Object.keys(filters).filter(key => 
      filters[key] !== null && filters[key] !== undefined && filters[key] !== ''
    );
    
    if (filterKeys.length === 0) return sortedData;

    return sortedData.filter(row => {
      return filterKeys.every(key => {
        const rowValue = row[key];
        const filterValue = filters[key];
        
        if (typeof rowValue === 'string' && typeof filterValue === 'string') {
          return rowValue.toLowerCase().includes(filterValue.toLowerCase());
        }
        
        return rowValue === filterValue;
      });
    });
  };

  renderTableHeader = (fields: any[]) => {
    const { sortable, showActions } = this.props;
    const { sortField, sortDirection } = this.state;

    return (
      <thead>
        <tr>
          {fields.map(field => (
            <th 
              key={field.name}
              className={sortable && field.sortable ? 'sortable' : ''}
              onClick={sortable && field.sortable ? () => this.handleSort(field.name) : undefined}
              style={sortable && field.sortable ? { cursor: 'pointer' } : {}}
            >
              {field.label}
              {sortable && field.sortable && sortField === field.name && (
                <span className="ms-1">
                  {sortDirection === 'asc' ? '↑' : '↓'}
                </span>
              )}
            </th>
          ))}
          {showActions && <th>Actions</th>}
        </tr>
      </thead>
    );
  };

  renderTableRow = (row: ListData, index: number, fields: any[]) => {
    const { onRowClick, onEdit, onDelete, onView, showActions } = this.props;

    return (
      <tr 
        key={index}
        className={onRowClick ? 'clickable-row' : ''}
        onClick={onRowClick ? () => onRowClick(row, index) : undefined}
        style={onRowClick ? { cursor: 'pointer' } : {}}
      >
        {fields.map(field => (
          <td key={field.name}>
            <Field
              field={field}
              value={row[field.name]}
              mode={MODES.LIST}
            />
          </td>
        ))}
        {showActions && (
          <td>
            <div className="btn-group btn-group-sm">
              {onView && (
                <button 
                  type="button"
                  className="btn btn-outline-primary btn-sm"
                  onClick={(e) => {
                    e.stopPropagation();
                    onView(row, index);
                  }}
                >
                  View
                </button>
              )}
              {onEdit && (
                <button 
                  type="button"
                  className="btn btn-outline-secondary btn-sm"
                  onClick={(e) => {
                    e.stopPropagation();
                    onEdit(row, index);
                  }}
                >
                  Edit
                </button>
              )}
              {onDelete && (
                <button 
                  type="button"
                  className="btn btn-outline-danger btn-sm"
                  onClick={(e) => {
                    e.stopPropagation();
                    if (window.confirm('Are you sure you want to delete this item?')) {
                      onDelete(row, index);
                    }
                  }}
                >
                  Delete
                </button>
              )}
            </div>
          </td>
        )}
      </tr>
    );
  };

  renderPagination = () => {
    const { pagination } = this.props;
    if (!pagination) return null;

    const { page, limit, total, onPageChange } = pagination;
    const totalPages = Math.ceil(total / limit);
    
    if (totalPages <= 1) return null;

    const pages: React.ReactElement[] = [];
    for (let i = 1; i <= totalPages; i++) {
      pages.push(
        <li key={i} className={`page-item ${page === i ? 'active' : ''}`}>
          <button 
            className="page-link"
            onClick={() => onPageChange(i)}
          >
            {i}
          </button>
        </li>
      );
    }

    return (
      <nav className="mt-3">
        <ul className="pagination justify-content-center">
          <li className={`page-item ${page === 1 ? 'disabled' : ''}`}>
            <button 
              className="page-link"
              onClick={() => onPageChange(Math.max(1, page - 1))}
              disabled={page === 1}
            >
              Previous
            </button>
          </li>
          {pages}
          <li className={`page-item ${page === totalPages ? 'disabled' : ''}`}>
            <button 
              className="page-link"
              onClick={() => onPageChange(Math.min(totalPages, page + 1))}
              disabled={page === totalPages}
            >
              Next
            </button>
          </li>
        </ul>
      </nav>
    );
  };

  render() {
    const { 
      fieldsetContext, 
      className,
      filterable 
    } = this.props;

    if (fieldsetContext.loading) {
      return (
        <div className="d-flex justify-content-center p-4">
          <div className="spinner-border" role="status">
            <span className="sr-only">Loading...</span>
          </div>
        </div>
      );
    }

    if (fieldsetContext.error) {
      return (
        <div className="alert alert-danger">
          Failed to load list configuration: {fieldsetContext.error}
        </div>
      );
    }

    if (!fieldsetContext.fieldset) {
      return (
        <div className="alert alert-warning">
          No list configuration available
        </div>
      );
    }

    const fields = fieldsetContext.getFieldsForMode();
    const listFields = fields.filter((field: any) => 
      field.mode === undefined || (field.mode & MODES.LIST) !== 0
    );
    
    const filteredData = this.getFilteredData();

    return (
      <div className={`fieldset-list ${className}`}>
        {filterable && (
          <div className="mb-3">
            {/* Filter inputs would go here */}
          </div>
        )}
        
        <div className="table-responsive">
          <table className="table table-striped table-hover">
            {this.renderTableHeader(listFields)}
            <tbody>
              {filteredData.length === 0 ? (
                <tr>
                  <td colSpan={listFields.length + (this.props.showActions ? 1 : 0)} className="text-center text-muted">
                    No data available
                  </td>
                </tr>
              ) : (
                filteredData.map((row, index) => 
                  this.renderTableRow(row, index, listFields)
                )
              )}
            </tbody>
          </table>
        </div>
        
        {this.renderPagination()}
      </div>
    );
  }
}

export default FieldsetListWrapper;