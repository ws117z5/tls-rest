import React, { Component } from "react";
import ReactDOM from "react-dom";
import StateList from "./States/List";

class UserList extends Component {

    render() {
        return (
            <StateList data={this.props.data} fieldset={this.props.fieldset} fieldsSelected={this.props.fieldsSelected}></StateList>
        )
    }
}

export default UserList;