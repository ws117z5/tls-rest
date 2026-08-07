import React from "react";

interface Option {
    name: string;
    value?: string | number;
}

interface SelectViewProps {
    value?: string | number;
    options?: Option[];
    width?: string | number;
    height?: string | number;
    fontFamily?: string;
    fontSize?: string | number;
    style?: React.CSSProperties;
}

const SelectView: React.FC<SelectViewProps> = ({
    value,
    options = [],
    width = "auto",
    height = "auto",
    fontFamily = "inherit",
    fontSize = "1rem",
    style = {},
}) => {
    const selected = options.find(opt => opt.value === value);
    return (
        <div
            style={{
                width,
                height,
                fontFamily,
                fontSize,
                overflow: "auto",
                ...style,
            }}
        >
            {selected ? selected.name : ""}
        </div>
    );
};

export default SelectView;