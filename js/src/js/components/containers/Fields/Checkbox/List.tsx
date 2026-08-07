import React, { Component } from "react";

//todo
class CheckboxList extends Component<BooleanProps> {

    render() {
        return (
            <input type="checkbox" checked={this.props.value} onChange={this.props.action} disabled={this.props.active}></input>
        )
    }
}

export default CheckboxList;