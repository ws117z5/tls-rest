import React from "react";

interface IntListProps {
    value?: number;
    format?: string;
    className?: string;
}

const IntList: React.FC<IntListProps> = ({ 
    value = 0, 
    format,
    className = ""
}) => {
    const formatValue = (val: number): string => {
        if (format === 'currency') {
            return new Intl.NumberFormat('en-US', { 
                style: 'currency', 
                currency: 'USD' 
            }).format(val);
        }
        if (format === 'percentage') {
            return `${val}%`;
        }
        return val.toLocaleString();
    };

    return (
        <span className={`int-list ${className}`}>
            {formatValue(value)}
        </span>
    );
};

export default IntList;