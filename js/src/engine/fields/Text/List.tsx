import React, { Component } from "react";

interface TextListProps {
    value?: string;
    maxLength?: number;
}

interface TextListState {
    showDialog: boolean;
}

class TextList extends Component<TextListProps, TextListState> {
    static defaultProps = {
        maxLength: 30,
        value: "",
    };

    state: TextListState = {
        showDialog: false,
    };

    handleOpen = () => {
        this.setState({ showDialog: true });
    };

    handleClose = () => {
        this.setState({ showDialog: false });
    };

    render() {
        const { value, maxLength } = this.props;
        const { showDialog } = this.state;
        const isLong = typeof value === "string" && value.length > (maxLength ?? 30);
        const displayValue = isLong ? value!.slice(0, maxLength) + "…" : value;

        return (
            <>
                <span
                    style={isLong ? { cursor: "pointer", textDecoration: "underline" } : {}}
                    onClick={isLong ? this.handleOpen : undefined}
                    title={isLong ? "Click to view full text" : undefined}
                >
                    {displayValue}
                </span>
                {showDialog && (
                    <div
                        style={{
                            position: "fixed",
                            top: 0,
                            left: 0,
                            width: "100vw",
                            height: "100vh",
                            background: "rgba(0,0,0,0.3)",
                            display: "flex",
                            alignItems: "center",
                            justifyContent: "center",
                            zIndex: 9999,
                        }}
                        onClick={this.handleClose}
                    >
                        <div
                            style={{
                                background: "#fff",
                                padding: "20px",
                                borderRadius: "8px",
                                maxWidth: "80vw",
                                maxHeight: "80vh",
                                overflow: "auto",
                                boxShadow: "0 2px 8px rgba(0,0,0,0.2)",
                            }}
                            onClick={e => e.stopPropagation()}
                        >
                            <div style={{ marginBottom: "1em" }}>{value}</div>
                            <button onClick={this.handleClose}>Close</button>
                        </div>
                    </div>
                )}
            </>
        );
    }
}

export default TextList;