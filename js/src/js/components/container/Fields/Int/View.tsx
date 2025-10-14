import React from "react";

interface IntViewProps {
    value?: number;
    label?: string;
    format?: string; // Format for display (e.g., currency, percentage)
    className?: string;
}

const IntView: React.FC<IntViewProps> = ({ 
    value = 0, 
    label,
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
        <div className={`int-view ${className}`}>
            {label && <label className="field-label">{label}</label>}
            <div className="field-content">
                <span className="int-value">
                    {formatValue(value)}
                </span>
            </div>
        </div>
    );
};

export default IntView;