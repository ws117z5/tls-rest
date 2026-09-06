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
  onRowDoubleClick?: (row: ListData, index: number) => void;
  onEdit?: (row: ListData, index: number) => void;
  onDelete?: (row: ListData, index: number) => void;
  // Bulk delete of the selected rows. Falls back to calling onDelete per row.
  onBulkDelete?: (rows: ListData[]) => void;
  onView?: (row: ListData, index: number) => void;
  className?: string;
  showActions?: boolean;
  sortable?: boolean;
  filterable?: boolean;
  // Enable the row-selection column + column chooser (default true).
  selectable?: boolean;
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
  selected: Set<string>;         // selected row ids
  hiddenColumns: Set<string>;    // column names hidden via the chooser
  showColumnMenu: boolean;
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
    selectable: true,
  };

  constructor(props: FieldsetListClassProps) {
    super(props);
    this.state = {
      sortField: undefined,
      sortDirection: 'asc',
      filters: {},
      selected: new Set<string>(),
      hiddenColumns: this.loadHidden(props.fieldsetContext?.module),
      showColumnMenu: false,
    };
  }

  componentDidMount() {
    document.addEventListener('click', this.handleDocClick);
  }

  componentWillUnmount() {
    document.removeEventListener('click', this.handleDocClick);
  }

  // Close the column menu on any outside click.
  handleDocClick = (e: MouseEvent) => {
    if (!this.state.showColumnMenu) return;
    const target = e.target as HTMLElement;
    if (!target.closest('.fieldset-columns-menu')) {
      this.setState({ showColumnMenu: false });
    }
  };

  // --- column visibility preference (localStorage; never sent to the backend) ---

  columnsKey() {
    return `fieldset-columns:${this.props.fieldsetContext?.module || 'default'}`;
  }

  loadHidden(module?: string): Set<string> {
    try {
      const raw = localStorage.getItem(`fieldset-columns:${module || 'default'}`);
      if (raw) return new Set<string>(JSON.parse(raw));
    } catch {
      /* ignore malformed / unavailable storage */
    }
    return new Set<string>();
  }

  saveHidden(hidden: Set<string>) {
    try {
      localStorage.setItem(this.columnsKey(), JSON.stringify(Array.from(hidden)));
    } catch {
      /* ignore */
    }
  }

  toggleColumn = (name: string) => {
    this.setState(s => {
      const hidden = new Set(s.hiddenColumns);
      if (hidden.has(name)) hidden.delete(name);
      else hidden.add(name);
      this.saveHidden(hidden);
      return { hiddenColumns: hidden };
    });
  };

  // --- selection ---

  rowId = (row: ListData, index: number): string =>
    row && row.id != null ? String(row.id) : `idx-${index}`;

  toggleRow = (id: string) => {
    this.setState(s => {
      const selected = new Set(s.selected);
      if (selected.has(id)) selected.delete(id);
      else selected.add(id);
      return { selected };
    });
  };

  toggleAll = (rows: ListData[]) => {
    this.setState(s => {
      const ids = rows.map((r, i) => this.rowId(r, i));
      const allSelected = ids.length > 0 && ids.every(id => s.selected.has(id));
      const selected = new Set(s.selected);
      if (allSelected) ids.forEach(id => selected.delete(id));
      else ids.forEach(id => selected.add(id));
      return { selected };
    });
  };

  clearSelection = () => this.setState({ selected: new Set<string>() });

  handleBulkDelete = (rows: ListData[]) => {
    const { onBulkDelete, onDelete } = this.props;
    const selectedRows: ListData[] = [];
    rows.forEach((r, i) => {
      if (this.state.selected.has(this.rowId(r, i))) selectedRows.push(r);
    });
    if (selectedRows.length === 0) return;
    if (!window.confirm(`Delete ${selectedRows.length} selected item(s)?`)) return;

    if (onBulkDelete) {
      onBulkDelete(selectedRows);
    } else if (onDelete) {
      selectedRows.forEach((r, i) => onDelete(r, i));
    }
    this.clearSelection();
  };

  // --- sorting / filtering (unchanged) ---

  handleSort = (fieldName: string) => {
    const { sortable } = this.props;
    if (!sortable) return;
    const { sortField, sortDirection } = this.state;
    let newDirection: 'asc' | 'desc' = 'asc';
    if (sortField === fieldName && sortDirection === 'asc') newDirection = 'desc';
    this.setState({ sortField: fieldName, sortDirection: newDirection });
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
    const filterKeys = Object.keys(filters).filter(
      key => filters[key] !== null && filters[key] !== undefined && filters[key] !== ''
    );
    if (filterKeys.length === 0) return sortedData;
    return sortedData.filter(row =>
      filterKeys.every(key => {
        const rowValue = row[key];
        const filterValue = filters[key];
        if (typeof rowValue === 'string' && typeof filterValue === 'string') {
          return rowValue.toLowerCase().includes(filterValue.toLowerCase());
        }
        return rowValue === filterValue;
      })
    );
  };

  // --- rendering ---

  renderToolbar = (allListFields: any[], rows: ListData[]) => {
    const { selectable } = this.props;
    const { selected, hiddenColumns, showColumnMenu } = this.state;

    return (
      <div className="d-flex justify-content-between align-items-center mb-2">
        <div>
          {selectable && selected.size > 0 && (
            <div className="btn-group btn-group-sm" role="group">
              <button
                type="button"
                className="btn btn-danger"
                onClick={() => this.handleBulkDelete(rows)}
              >
                Delete ({selected.size})
              </button>
              <button type="button" className="btn btn-outline-secondary" onClick={this.clearSelection}>
                Clear
              </button>
            </div>
          )}
        </div>

        {selectable && (
          <div className="fieldset-columns-menu position-relative">
            <button
              type="button"
              className="btn btn-outline-secondary btn-sm"
              onClick={() => this.setState(s => ({ showColumnMenu: !s.showColumnMenu }))}
            >
              Columns
            </button>
            {showColumnMenu && (
              <div
                className="card shadow-sm p-2 position-absolute end-0"
                style={{ zIndex: 1000, minWidth: 200, maxHeight: 320, overflow: 'auto' }}
              >
                {allListFields.map((f: any) => (
                  <label key={f.name} className="d-flex align-items-center gap-2 py-1 mb-0 small">
                    <input
                      type="checkbox"
                      checked={!hiddenColumns.has(f.name)}
                      onChange={() => this.toggleColumn(f.name)}
                    />
                    {f.label || f.name}
                  </label>
                ))}
              </div>
            )}
          </div>
        )}
      </div>
    );
  };

  renderTableHeader = (fields: any[], rows: ListData[]) => {
    const { sortable, showActions, selectable } = this.props;
    const { sortField, sortDirection, selected } = this.state;

    const ids = rows.map((r, i) => this.rowId(r, i));
    const allSelected = ids.length > 0 && ids.every(id => selected.has(id));
    const someSelected = ids.some(id => selected.has(id));

    return (
      <thead>
        <tr>
          {selectable && (
            <th style={{ width: 36 }}>
              <input
                type="checkbox"
                aria-label="Select all"
                checked={allSelected}
                ref={el => {
                  if (el) el.indeterminate = someSelected && !allSelected;
                }}
                onChange={() => this.toggleAll(rows)}
              />
            </th>
          )}
          {fields.map(field => (
            <th
              key={field.name}
              className={sortable && field.sortable ? 'sortable' : ''}
              onClick={sortable && field.sortable ? () => this.handleSort(field.name) : undefined}
              style={sortable && field.sortable ? { cursor: 'pointer' } : {}}
            >
              {field.label}
              {sortable && field.sortable && sortField === field.name && (
                <span className="ms-1">{sortDirection === 'asc' ? '↑' : '↓'}</span>
              )}
            </th>
          ))}
          {showActions && <th>Actions</th>}
        </tr>
      </thead>
    );
  };

  renderTableRow = (row: ListData, index: number, fields: any[]) => {
    const { onRowClick, onRowDoubleClick, onEdit, onDelete, onView, showActions, selectable } = this.props;
    const id = this.rowId(row, index);
    const isSelected = this.state.selected.has(id);

    return (
      <tr
        key={id}
        className={`${(onRowClick || onRowDoubleClick) ? 'clickable-row' : ''} ${isSelected ? 'table-active' : ''}`}
        onClick={onRowClick ? () => onRowClick(row, index) : undefined}
        onDoubleClick={onRowDoubleClick ? () => onRowDoubleClick(row, index) : undefined}
        style={(onRowClick || onRowDoubleClick) ? { cursor: 'pointer' } : {}}
      >
        {selectable && (
          <td onClick={e => e.stopPropagation()}>
            <input
              type="checkbox"
              aria-label="Select row"
              checked={isSelected}
              onChange={() => this.toggleRow(id)}
            />
          </td>
        )}
        {fields.map(field => (
          <td key={field.name}>
            <Field field={field} value={row[field.name]} mode={MODES.LIST} />
          </td>
        ))}
        {showActions && (
          <td>
            <div className="btn-group btn-group-sm">
              {onView && (
                <button
                  type="button"
                  className="btn btn-outline-primary btn-sm"
                  onClick={e => {
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
                  onClick={e => {
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
                  onClick={e => {
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

    // Windowed page list: first, current ± radius, last, with ellipses for gaps.
    const radius = 3;
    const windowSize = radius * 2 + 1;
    let lo = Math.max(1, page - radius);
    let hi = Math.min(totalPages, lo + windowSize - 1);
    lo = Math.max(1, hi - windowSize + 1); // re-clamp near the end

    const items: Array<number | "…"> = [];
    if (lo > 1) {
      items.push(1);
      if (lo > 2) items.push("…");
    }
    for (let i = lo; i <= hi; i++) items.push(i);
    if (hi < totalPages) {
      if (hi < totalPages - 1) items.push("…");
      items.push(totalPages);
    }

    const pages = items.map((it, idx) =>
      it === "…" ? (
        <li key={`gap-${idx}`} className="page-item disabled">
          <span className="page-link">…</span>
        </li>
      ) : (
        <li key={it} className={`page-item ${page === it ? "active" : ""}`}>
          <button className="page-link" onClick={() => onPageChange(it)}>{it}</button>
        </li>
      )
    );

    return (
      <nav className="mt-3">
        <ul className="pagination justify-content-center">
          <li className={`page-item ${page === 1 ? 'disabled' : ''}`}>
            <button className="page-link" onClick={() => onPageChange(Math.max(1, page - 1))} disabled={page === 1}>
              Previous
            </button>
          </li>
          {pages}
          <li className={`page-item ${page === totalPages ? 'disabled' : ''}`}>
            <button className="page-link" onClick={() => onPageChange(Math.min(totalPages, page + 1))} disabled={page === totalPages}>
              Next
            </button>
          </li>
        </ul>
      </nav>
    );
  };

  render() {
    const { fieldsetContext, className, filterable, showActions, selectable } = this.props;

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
      return <div className="alert alert-warning">No list configuration available</div>;
    }

    const fields = fieldsetContext.getFieldsForMode();
    const allListFields = fields.filter(
      (field: any) => field.mode === undefined || (field.mode & MODES.LIST) !== 0
    );
    // Apply the (local-only) column visibility preference.
    const listFields = allListFields.filter((f: any) => !this.state.hiddenColumns.has(f.name));

    const filteredData = this.getFilteredData();
    const leadingCols = selectable ? 1 : 0;

    return (
      <div className={`fieldset-list ${className}`}>
        {this.renderToolbar(allListFields, filteredData)}

        {filterable && <div className="mb-3">{/* Filter inputs would go here */}</div>}

        <div className="table-responsive">
          <table className="table table-striped table-hover">
            {this.renderTableHeader(listFields, filteredData)}
            <tbody>
              {filteredData.length === 0 ? (
                <tr>
                  <td
                    colSpan={listFields.length + leadingCols + (showActions ? 1 : 0)}
                    className="text-center text-muted"
                  >
                    No data available
                  </td>
                </tr>
              ) : (
                filteredData.map((row, index) => this.renderTableRow(row, index, listFields))
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