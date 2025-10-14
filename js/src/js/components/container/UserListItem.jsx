import React, { Component } from "react";
import ReactDOM from "react-dom";

class UserList extends Component { 

    render() {
        return (
            <ul className="users-list">
                {this.props.users.map(user => 
                    <li>{user.name}</li>
                )}
            </ul>
        )
    }
}

export default UserList;