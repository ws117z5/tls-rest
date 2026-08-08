import React from "react";

interface ButtonViewProps {
    id?: string;
    className?: string;
    width?: string | number;
    height?: string | number;
    action?: (e: React.MouseEvent<HTMLButtonElement, MouseEvent>) => void;
    children?: React.ReactNode;
    style?: React.CSSProperties;
}

const defaultProps: Partial<ButtonViewProps> = {
    width: "120px",
    height: "40px",
    className: "",
    action: () => {},
};

const ButtonView: React.FC<ButtonViewProps> = (props) => {
    const {
        id,
        className,
        width,
        height,
        action,
        children,
        style,
        ...rest
    } = { ...defaultProps, ...props };

    return (
        <button
            id={id}
            className={className}
            style={{
                width,
                height,
                transition: "transform 0.2s, box-shadow 0.2s",
                ...style,
            }}
            onClick={action}
            onMouseDown={e => {
                (e.currentTarget as HTMLButtonElement).style.transform = "scale(0.96)";
                (e.currentTarget as HTMLButtonElement).style.boxShadow = "0 2px 8px rgba(0,0,0,0.15)";
            }}
            onMouseUp={e => {
                (e.currentTarget as HTMLButtonElement).style.transform = "scale(1)";
                (e.currentTarget as HTMLButtonElement).style.boxShadow = "";
            }}
            onMouseLeave={e => {
                (e.currentTarget as HTMLButtonElement).style.transform = "scale(1)";
                (e.currentTarget as HTMLButtonElement).style.boxShadow = "";
            }}
            {...rest}
        >
            {children || "Button"}
        </button>
    );
};

export default ButtonView;