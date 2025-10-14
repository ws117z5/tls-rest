import React, { Component } from "react";
import ReactDOM from "react-dom";
import axios from 'axios';
import AuthButton from "./AuthButton.jsx";


class Login extends Component {
  // Use the render function to return JSX component

  constructor(props) {
    super(props);
  }

  loginVk() {

    /*
    window.location = "https://oauth.vk.com/authorize?client_id=6606830&redirect_uri=https://localhost/users/Auth/Vk&display=mobile&response_type=token";
    */
   console.log(this);

  };

  loginGoogle() {
    window.location = "https://localhost/users/Auth/GoogleLogin"
  }

  componentDidMount() {
    
  }

  render() {
    return (
      <div className="shopping-list">
        <h1>Login using: </h1>
        <ul>
          <li><AuthButton name="Facebook" call={this.loginGoogle}></AuthButton></li>
          <li><AuthButton name="Google" call={this.loginGoogle}></AuthButton></li>
        </ul>
      </div>
    );
  }
}

export default Login;