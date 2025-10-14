'use strict'

import React, { Component } from "react";
import ReactDOM from "react-dom";
import Login from "../container/LoginContainer.jsx";
import PageComponent from "../../controllers/PageComponent";

// Create a function to wrap up your component
class LoginPage extends PageComponent {
  static href = 'login'
  static title = 'Login'

    render() {
    return (
      <div className="base">
        <Login name="login" />
      </div>
    )
  }
}

export default LoginPage;