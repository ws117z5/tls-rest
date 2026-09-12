import React, { Component } from "react";
import OpenCV from "./containers/OpenCV";
import ReactDOM from "react-dom";
import PageComponent from "@engine/containers/PageComponent";
import { t, subscribe } from "@engine/i18n";

// Create a function to wrap up your component
class OpenCVPage extends PageComponent {
  protected href = 'opencv'
  protected isPage = true;
  protected title = 'Open CV'
  protected submenu = "tools";

  private unsubscribeI18n?: () => void;
  async componentDidMount() {
    await super.componentDidMount();
    this.unsubscribeI18n = subscribe(() => this.forceUpdate());
  }
  componentWillUnmount() {
    this.unsubscribeI18n?.();
  }

  render() {
    return (
      <div className="base">
        {t("This is OpenCV webrtc to gocv implementation")}
        <OpenCV></OpenCV>
      </div>
    )
  }
}

export default OpenCVPage;