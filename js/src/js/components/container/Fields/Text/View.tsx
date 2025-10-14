import React, { Component, CSSProperties } from "react";
import PropTypes from "prop-types";

interface TextViewProps {
    value?: string;
    width?: string | number;
    height?: string | number;
    fontFamily?: string;
    fontSize?: string | number;
    style?: CSSProperties;
}

class TextView extends Component<TextViewProps> {
    propTypes = {
        value: PropTypes.string,
        width: PropTypes.oneOfType([PropTypes.string, PropTypes.number]),
        height: PropTypes.oneOfType([PropTypes.string, PropTypes.number]),
        fontFamily: PropTypes.string,
        fontSize: PropTypes.oneOfType([PropTypes.string, PropTypes.number]),
        style: PropTypes.object,
    };

    static defaultProps = {
        width: "auto",
        height: "auto",
        fontFamily: "inherit",
        fontSize: "1rem",
        value: "",
    };

    render() {
        const { value, width, height, fontFamily, fontSize, style = {} } = this.props;
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
                {value}
            </div>
        );
    }
}

export default TextView;