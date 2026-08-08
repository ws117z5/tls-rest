import React, { Component } from "react";
import ReactDOM from "react-dom";

interface AuthButtonProps {
    name: string;
    call: () => void;
}

class AuthButton extends Component<AuthButtonProps> {
    render() {
        return (
            <button className="auth-button" onClick={this.props.call}>
                {this.props.name}
            </button>
        );
    }
}


export default AuthButton;