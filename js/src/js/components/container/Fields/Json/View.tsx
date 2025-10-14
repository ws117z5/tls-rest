import React from "react";

interface JsonViewProps {
    value?: object | string;
    label?: string;
    maxHeight?: number;
    collapsed?: boolean;
    className?: string;
}

const JsonView: React.FC<JsonViewProps> = ({ 
    value = {}, 
    label,
    maxHeight = 300,
    collapsed = false,
    className = ""
}) => {
    const [isCollapsed, setIsCollapsed] = React.useState(collapsed);
    
    const jsonString = typeof value === 'string' 
        ? value 
        : JSON.stringify(value, null, 2);

    const toggleCollapse = () => {
        setIsCollapsed(!isCollapsed);
    };

    return (
        <div className={`json-view ${className}`}>
            {label && (
                <div className="d-flex justify-content-between align-items-center">
                    <label className="field-label">{label}</label>
                    <button 
                        type="button"
                        className="btn btn-sm btn-outline-secondary"
                        onClick={toggleCollapse}
                    >
                        {isCollapsed ? 'Show' : 'Hide'}
                    </button>
                </div>
            )}
            {!isCollapsed && (
                <div className="field-content">
                    <pre 
                        className="bg-light p-2 border rounded"
                        style={{ 
                            maxHeight: `${maxHeight}px`, 
                            overflow: 'auto',
                            fontSize: '0.85rem'
                        }}
                    >
                        <code>{jsonString}</code>
                    </pre>
                </div>
            )}
            {isCollapsed && (
                <div className="field-content">
                    <span className="text-muted">
                        {typeof value === 'object' ? `{${Object.keys(value).length} properties}` : 'JSON data'}
                    </span>
                </div>
            )}
        </div>
    );
};

export default JsonView;