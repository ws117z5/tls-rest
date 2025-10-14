import React, { Component } from "react";
import ReactDOM from "react-dom";

class AuthButton extends React.Component {
    render() {
        return (
            <button className="auth-button" onClick={this.props.call}>
                {this.props.name}
            </button>
        );
    }
}


export default AuthButton;