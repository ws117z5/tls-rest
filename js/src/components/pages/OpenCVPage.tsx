import React, { Component } from "react";
import OpenCV from "@containers/OpenCV/OpenCV";
import ReactDOM from "react-dom";
import PageComponent from "@engine/PageComponent";

// Create a function to wrap up your component
class OpenCVPage extends PageComponent {
  protected href = 'opencv'
  protected isPage = true;
  protected title = 'Open CV'
  
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