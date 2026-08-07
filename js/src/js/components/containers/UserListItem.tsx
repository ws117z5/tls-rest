import React, { Component } from "react";

interface UserListProps {
    users: { name: string }[];
}   

class UserList extends Component<UserListProps> { 

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