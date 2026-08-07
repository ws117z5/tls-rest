import React, { Component } from "react";
import ReactDOM from "react-dom";

class PostList extends Component {

    render() {
        return (
            <ul className="post-list">
                {this.props.data.map(post => 
                    <li>{post.name}</li>
                )}
            </ul>
        )
    }
}

export default PostList;