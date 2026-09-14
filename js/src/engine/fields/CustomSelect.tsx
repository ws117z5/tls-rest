import React, { useEffect, useRef, useState } from "react";

// A <select> replacement matching the language switcher's design: the current
// value stays in place as a rounded control, and clicking it expands the SAME
// box downward to reveal the other options — no separate floating menu, no
// native <select> box (which the served bootstrap.min.css v4 doesn't style
// via form-select/form-control anyway; see engine/containers/Menu.tsx).
//
// Classes are prefixed "cx-select" (not "custom-select") because Bootstrap 4
// already defines its own native-<select> component under that exact class
// name — a same-named div here inherited its background-image/appearance/
// padding rules and produced a hybrid native-plus-custom look.

export interface CustomSelectOption {
    value: string;
    label: string;
}

interface CustomSelectProps {
    value: string;
    options: CustomSelectOption[];
    onChange: (value: string) => void;
    disabled?: boolean;
    placeholder?: string; // shown when value matches no option
    className?: string;
    id?: string;
}

const CustomSelect: React.FC<CustomSelectProps> = ({
    value,
    options,
    onChange,
    disabled,
    placeholder,
    className,
    id,
}) => {
    const [open, setOpen] = useState(false);
    const wrapRef = useRef<HTMLDivElement>(null);

    useEffect(() => {
        if (!open) return;
        const onDocClick = (e: MouseEvent) => {
            if (wrapRef.current && !wrapRef.current.contains(e.target as Node)) setOpen(false);
        };
        document.addEventListener("click", onDocClick);
        return () => document.removeEventListener("click", onDocClick);
    }, [open]);

    const current = options.find((o) => o.value === value);
    const others = options.filter((o) => o.value !== value);

    return (
        <div
            ref={wrapRef}
            className={`cx-select ${open ? "cx-select-open" : ""} ${disabled ? "cx-select-disabled" : ""} ${className || ""}`}
        >
            <button
                type="button"
                id={id}
                className="cx-select-current"
                disabled={disabled}
                onClick={() => setOpen((o) => !o)}
                aria-expanded={open}
            >
                <span className="cx-select-current-label">
                    {current ? current.label : placeholder || ""}
                </span>
                <span className="cx-select-caret" aria-hidden="true" />
            </button>
            <div className="cx-select-options">
                {others.map((o) => (
                    <button
                        type="button"
                        key={o.value}
                        className="cx-select-option"
                        onClick={() => {
                            onChange(o.value);
                            setOpen(false);
                        }}
                    >
                        {o.label}
                    </button>
                ))}
            </div>
        </div>
    );
};

export default CustomSelect;
