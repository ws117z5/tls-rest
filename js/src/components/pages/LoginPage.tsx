'use strict'

import React, { Component } from "react";
import ReactDOM from "react-dom";
import Login from "@containers/LoginContainer";
import PageComponent from "@engine/PageComponent";

// Create a function to wrap up your component
class LoginPage extends PageComponent {
  protected href = 'login'
  protected title = 'Login'

    render() {
    return (
      <div className="base">
        <Login name="login" />
      </div>
    )
  }
}

export default LoginPage;