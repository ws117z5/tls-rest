# Fieldset-Based Component System

This system provides automatic form generation, list display, and CRUD operations based on fieldset configurations defined in the Go backend.

## Overview

The fieldset system consists of:

1. **Backend Fieldset Engine** - Automatic SQL generation and API endpoints
2. **React Field Components** - Type-specific UI components 
3. **Form & List Components** - Complete CRUD interfaces
4. **Provider System** - Context-based fieldset management

## Usage

### 1. Define Backend Module

```go
// Define your module with fieldsets
var PostsModule = &ModuleAbstract[interface{}]{
    ID:   "posts",
    Name: "Posts",
    Fields: []Field{
        NewField("title", TYPE_STRING, true).
            WithLabel("Title").
            WithValidation("minLength", 3).
            WithValidation("maxLength", 200),

        NewField("content", TYPE_TEXT, true).
            WithLabel("Content").
            WithValidation("minLength", 10),

        NewField("created", TYPE_DATE_TIME, false).
            WithSQL("now()").
            AsReadOnly().
            WithMode(MODE_LIST | MODE_VIEW),
    },
}

// Create controller with automatic CRUD
var controller = NewBaseController(PostsModule, "posts")

// Register for API access
func init() {
    GlobalFieldsetHandler.RegisterModule(PostsModule)
}
```

### 2. Use in React Components

```tsx
import { 
    FieldsetProvider, 
    FieldsetForm, 
    FieldsetList, 
    MODES 
} from './components/container/Fields';

// Wrap your component with FieldsetProvider
<FieldsetProvider module="posts" mode={MODES.EDIT}>
    <FieldsetForm
        data={postData}
        onSubmit={handleSubmit}
        onChange={handleFieldChange}
    />
</FieldsetProvider>

// Display data in a list
<FieldsetProvider module="posts" mode={MODES.LIST}>
    <FieldsetList
        data={posts}
        onEdit={handleEdit}
        onDelete={handleDelete}
        pagination={{
            page: 1,
            limit: 20,
            total: 100,
            onPageChange: handlePageChange
        }}
    />
</FieldsetProvider>
```

### 3. Individual Field Components

```tsx
import { Field, MODES } from './components/container/Fields';

// Render individual field
<Field
    field={{
        name: 'title',
        type: 'String',
        label: 'Post Title',
        required: true
    }}
    value={title}
    onChange={setTitle}
    mode={MODES.EDIT}
/>
```

## Available Field Types

### Basic Types
- `String` - Text input
- `Text` - Textarea
- `Int` - Number input
- `Float` - Decimal input
- `Date` - Date picker
- `DateTime` - Date and time picker

### Advanced Types
- `Select` - Dropdown selection
- `Checkbox` - Boolean checkbox
- `Json` - JSON editor
- `Autocomplete` - Text input with suggestions
- `Money` - Currency input

### Display Types
- `Html` - Rich text display
- `Color` - Color picker
- `Week` - Week selector
- `Month` - Month selector

## Field Configuration

```go
NewField("fieldName", TYPE_STRING, required).
    WithLabel("Display Label").
    WithDescription("Helper text").
    WithValidation("minLength", 5).
    WithValidation("maxLength", 100).
    WithOption("autocompleteUrl", "/api/suggestions").
    WithMode(MODE_EDIT | MODE_VIEW).
    AsReadOnly().
    NonFilterable()
```

### Validation Options
- `minLength`, `maxLength` - String length
- `min`, `max` - Numeric range  
- `email` - Email validation
- `pattern` - Regex pattern
- `patternMessage` - Custom error message

### Field Modes
- `MODE_LIST` - Show in list view
- `MODE_VIEW` - Show in view mode
- `MODE_EDIT` - Show in edit mode  
- `MODE_ALL` - Show in all modes

## API Endpoints

The system automatically provides:

### Fieldset Configuration
```
GET /api/modules/{moduleId}/fieldset?mode={mode}
```

Returns field definitions for the specified module and mode.

### CRUD Operations
```
GET /api/{module}                    # List with pagination/filtering
GET /api/{module}/{id}               # View single record
POST /api/{module}                   # Create new record
PUT /api/{module}/{id}               # Update record
DELETE /api/{module}/{id}            # Delete record
```

### Query Parameters
- `page` - Page number (default: 1)
- `limit` - Records per page (default: 20)
- `sort` - Sort field
- `order` - Sort direction (asc/desc)
- `search` - Full-text search
- `filters[field]` - Field-specific filters

## Examples

### Complete CRUD Page
See `PostsPage.tsx` for a complete example of:
- List view with pagination and sorting
- Create/Edit forms with validation
- View mode for read-only display
- Delete confirmation

### Field Customization
```tsx
// Custom field component
const CustomField: React.FC<BaseFieldProps> = ({ field, value, onChange }) => {
    return (
        <div className="custom-field">
            <label>{field.label}</label>
            <input 
                type="text"
                value={value || ''}
                onChange={(e) => onChange?.(e.target.value)}
            />
        </div>
    );
};

// Register custom field type
FIELD_COMPONENTS['CustomType'] = {
    [MODES.EDIT]: CustomField,
    [MODES.VIEW]: CustomField,
    [MODES.LIST]: CustomField,
};
```

## Benefits

1. **Automatic SQL Generation** - No more manual query writing
2. **Type Safety** - Field types ensure proper validation
3. **Consistent UI** - Standardized form and list components  
4. **Rapid Development** - Define schema once, get complete CRUD
5. **Extensible** - Easy to add new field types and validation rules
6. **Responsive** - Bootstrap-based responsive design
7. **Accessible** - Proper ARIA labels and keyboard navigation

## Migration from Manual Forms

1. Define your module fieldset in Go
2. Replace manual form JSX with `<FieldsetForm>`  
3. Replace table/list JSX with `<FieldsetList>`
4. Update API endpoints to use base controller
5. Remove manual SQL queries and validation logic

The system maintains backward compatibility with existing components while providing a path to modernize your forms incrementally.