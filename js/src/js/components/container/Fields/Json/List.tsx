import React from "react";

interface JsonListProps {
    value?: object | string;
    maxLength?: number;
    className?: string;
}

const JsonList: React.FC<JsonListProps> = ({ 
    value = {}, 
    maxLength = 50,
    className = ""
}) => {
    const jsonString = typeof value === 'string' 
        ? value 
        : JSON.stringify(value);
    
    const displayString = jsonString.length > maxLength 
        ? jsonString.substring(0, maxLength) + '...'
        : jsonString;

    return (
        <span 
            className={`json-list ${className}`}
            title={jsonString} // Full JSON on hover
        >
            <code className="text-muted">{displayString}</code>
        </span>
    );
};

export default JsonList;