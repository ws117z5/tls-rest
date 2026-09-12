import React, { Component } from "react";
import ReactDOM from "react-dom";
import PageComponent from "@engine/containers/PageComponent"
import { t, subscribe } from "@engine/i18n";

// Create a function to wrap up your component
class DataPage extends PageComponent {
    protected href = 'data';
    protected isPage = false;

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
           {t("This is data")}
        </div>
        )
  }
}

export default DataPage;