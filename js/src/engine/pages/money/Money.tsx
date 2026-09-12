import React, { Component } from "react";
import ReactDOM from "react-dom";
import PageComponent, {PageComponentState} from "@engine/containers/PageComponent";
import { Navigate } from "react-router";
import { t, subscribe } from "@engine/i18n";

interface MoneyPageState extends PageComponentState {
    redirect: boolean;
}

class MoneyPage extends PageComponent<{}, MoneyPageState> {
    protected href = 'money'
    protected title = 'Money'
    
    constructor(props: any) {
        super(props);
        this.state = {
            ...this.state, // Preserves required Data and Fieldsetts from parent
            redirect: false
        };
    }

    private unsubscribeI18n?: () => void;
    async componentDidMount() {
        await super.componentDidMount();
        this.unsubscribeI18n = subscribe(() => this.forceUpdate());
    }
    componentWillUnmount() {
        this.unsubscribeI18n?.();
    }

    //double-check if authorized
    /*
    handleSubmit () {
      axios.post('')
        .then(() => this.setState({ redirect: true }));
    }
    */

    render () {
      const { redirect } = this.state;
      
        if (redirect) {
          return <Navigate to='/' />;
        }

        return  <div className="base">
            {t("This is money page")}
        </div>
    }
}

export default MoneyPage;