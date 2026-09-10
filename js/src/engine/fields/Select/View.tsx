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
    // Compare as strings: a FK id may arrive as a number from the record but as
    // a string in the option list (or vice versa), and 0 is a valid id.
    const selected =
        value === undefined || value === null || value === ""
            ? undefined
            : options.find(opt => String(opt.value ?? "") === String(value));
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