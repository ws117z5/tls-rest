import React from "react";

interface Option {
    name: string;
    value?: string | number;
}

interface SelectListProps {
    value?: string | number;
    options?: Option[];
}

const SelectList: React.FC<SelectListProps> = ({ value, options = [] }) => {
    const selected = options.find(opt => opt.value === value);
    return <span>{selected ? selected.name : ""}</span>;
};

export default SelectList;