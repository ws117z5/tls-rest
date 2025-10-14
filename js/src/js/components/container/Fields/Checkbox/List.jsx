import React, { Component } from "react";

//todo
class CheckboxList extends Component {

    render() {
        return (
            <input type="checkbox" value={this.props.value} action={this.props.action}></input>
        )
    }
}

export default CheckboxList;