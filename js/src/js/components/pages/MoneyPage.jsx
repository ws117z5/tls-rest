import React, { Component } from "react";
import ReactDOM from "react-dom";
import PageComponent from "../../controllers/PageComponent";

class MoneyPage extends PageComponent {
    static href = 'money'
    static title = 'Money'
    
    state = {
      redirect: false
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
          return <Redirect to='/somewhere'/>;
        }

        return  <div className="base">
            This is money page
        </div>
    }
}

export default MoneyPage;