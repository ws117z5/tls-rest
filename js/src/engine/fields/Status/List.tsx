import React from "react";

// Boolean rendered as a colored rounded pill (e.g. online/offline) instead of a checkbox — opt in via field.options.widget = "status".
const StatusList: React.FC<BooleanProps> = ({ value }) => {
    const on = !!value;
    return (
        <span
            style={{
                display: "inline-block",
                padding: "2px 10px",
                borderRadius: "10px",
                fontSize: "0.85em",
                color: "#fff",
                backgroundColor: on ? "#28a745" : "#6c757d",
            }}
        >
            {on ? "Online" : "Offline"}
        </span>
    );
};

export default StatusList;
