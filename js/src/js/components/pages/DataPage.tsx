import React, { Component } from "react";
import ReactDOM from "react-dom";
import PageComponent from "@controllers/PageComponent"

// Create a function to wrap up your component
class DataPage extends PageComponent {
    protected href = 'data';
    protected isPage = false;

    render() {
        return (
        <div className="base">
           This is data
        </div>
        )
  }
}

export default DataPage;