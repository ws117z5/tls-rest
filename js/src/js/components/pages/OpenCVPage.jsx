import React, { Component } from "react";
import OpenCV from "../container/OpenCV/OpenCV";
import ReactDOM from "react-dom";
import PageComponent from "../../controllers/PageComponent";

// Create a function to wrap up your component
class OpenCVPage extends PageComponent {
  static href = 'opencv'
  static isPage = true;
  static title = 'Open CV'
  
  render() {
    return (
      <div className="base">
        This is OpenCV webrtc to gocv implementation
        <OpenCV></OpenCV>
      </div>
    )
  }
}

export default OpenCVPage;