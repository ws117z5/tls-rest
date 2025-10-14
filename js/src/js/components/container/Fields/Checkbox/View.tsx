import React from "react";

interface CheckboxViewProps {
    value?: boolean;
    label?: string;
    className?: string;
}

const CheckboxView: React.FC<CheckboxViewProps> = ({ 
    value = false, 
    label,
    className = ""
}) => {
    return (
        <div className={`checkbox-view ${className}`}>
            {label && <label className="field-label">{label}</label>}
            <div className="field-content">
                <span className={`badge ${value ? 'badge-success' : 'badge-secondary'}`}>
                    {value ? '✓ Yes' : '✗ No'}
                </span>
            </div>
        </div>
    );
};

export default CheckboxView;