import React, { Component } from "react";
import PostList from "../container/PostList.jsx";
import Request from "../../controllers/request.jsx";
import PageComponent from "../../controllers/PageComponent";
import Config from "../../controllers/config";
import {NavLink} from 'reactstrap'

// Create a function to wrap up your component
class PostsPage extends PageComponent {
    static href = 'posts'
    static title = 'Posts'

    constructor(props) {
        super(props);
        this.state = { Data: [] };

    }
    
    componentDidMount() {
        Request.apiListRequest('posts', this);
    }
    
    render() {
        return (
            <div className="base">
                This is posts
                {
                    Config.getPages()?.map((Page, key) => {
                        return <NavLink key={key} href={'/pages/'+Page.href}>{Page.title}</NavLink>
                    })
                }
                <PostList data={this.state.Data}></PostList>
        </div>
        )
    }
}

export default PostsPage;