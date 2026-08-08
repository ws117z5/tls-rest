import React, { Component } from "react";
import ReactDOM from "react-dom";
import PageComponent from "@engine/PageComponent";

// Create a function to wrap up your component
class ProfilePage extends PageComponent {
  protected href = 'profile';
  protected title = 'Profile'

    render() {
        return (
        <div className="base">
           This is profile
        </div>
        )
  }
}

export default ProfilePage;