import { Component } from "react";
import StateList from "./States/List";
import { DefaultListProps } from "@engine/PageComponent";


class UserList extends Component<DefaultListProps> {

    render() {
        return (
            <StateList data={this.props.data} fieldset={this.props.fieldset} fieldsSelected={this.props.fieldsSelected}></StateList>
        )
    }
}

export default UserList;