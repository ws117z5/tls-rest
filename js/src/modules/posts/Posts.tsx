import React from 'react';
import PageComponent from '@engine/containers/PageComponent';
import { 
    FieldsetProvider, 
    FieldsetForm, 
    FieldsetList,
    MODES 
} from '@engine/fields';
import axios from 'axios';

interface PostsPageState {
    posts: any[];
    loading: boolean;
    currentView: 'list' | 'create' | 'edit' | 'view';
    selectedPost: any | null;
    pagination: {
        page: number;
        limit: number;
        total: number;
    };
}

class PostsPage extends PageComponent<{}, PostsPageState> {
    // Shown in the main nav menu (isPage=false), publicly accessible.
    protected isPage = false;
    protected href = 'posts';
    protected title = 'Posts';

    constructor(props: {}) {
        super(props);
        this.state = {
            posts: [],
            loading: true,
            currentView: 'list',
            selectedPost: null,
            pagination: {
                page: 1,
                limit: 20,
                total: 0
            }
        };
    }

    async componentDidMount() {
        this.loadPosts();
    }

    loadPosts = async (page: number = 1) => {
        try {
            this.setState({ loading: true });
            
            const response = await axios.get('/posts', {
                params: {
                    page,
                    limit: this.state.pagination.limit,
                    // Only display public posts (engine reads filters[<field>]).
                    'filters[public]': 1
                }
            });
            
            this.setState({
                posts: response.data.Data || [],
                loading: false,
                pagination: {
                    ...this.state.pagination,
                    page: response.data.Page || 1,
                    total: response.data.Total || 0
                }
            });
        } catch (error) {
            console.error('Failed to load posts:', error);
            this.setState({ loading: false });
        }
    };

    handleCreatePost = () => {
        this.setState({
            currentView: 'create',
            selectedPost: null
        });
    };

    handleEditPost = (post: any) => {
        this.setState({
            currentView: 'edit',
            selectedPost: post
        });
    };

    handleViewPost = (post: any) => {
        this.setState({
            currentView: 'view',
            selectedPost: post
        });
    };

    handleDeletePost = async (post: any) => {
        try {
            await axios.delete(`/posts/${post.id}`);
            this.loadPosts(this.state.pagination.page);
        } catch (error) {
            console.error('Failed to delete post:', error);
            alert('Failed to delete post');
        }
    };

    handleSubmitPost = async (data: any) => {
        try {
            if (this.state.currentView === 'create') {
                await axios.post('/posts', data);
            } else if (this.state.currentView === 'edit' && this.state.selectedPost) {
                await axios.put(`/posts/${this.state.selectedPost.id}`, data);
            }
            
            this.setState({ currentView: 'list' });
            this.loadPosts(this.state.pagination.page);
        } catch (error) {
            console.error('Failed to save post:', error);
            alert('Failed to save post');
        }
    };

    handleBackToList = () => {
        this.setState({
            currentView: 'list',
            selectedPost: null
        });
    };

    handlePageChange = (page: number) => {
        this.loadPosts(page);
    };

    renderListView() {
        const { posts, pagination } = this.state;

        return (
            <div>
                <div className="d-flex justify-content-between align-items-center mb-4">
                    <h1>Posts</h1>
                    <button 
                        className="btn btn-primary"
                        onClick={this.handleCreatePost}
                    >
                        Create New Post
                    </button>
                </div>

                <FieldsetProvider module="posts" mode={MODES.LIST}>
                    <FieldsetList
                        data={posts}
                        onView={this.handleViewPost}
                        onEdit={this.handleEditPost}
                        onDelete={this.handleDeletePost}
                        pagination={{
                            ...pagination,
                            onPageChange: this.handlePageChange
                        }}
                        sortable={true}
                        showActions={true}
                    />
                </FieldsetProvider>
            </div>
        );
    }

    renderFormView() {
        const { currentView, selectedPost } = this.state;
        const isEdit = currentView === 'edit';
        const isView = currentView === 'view';
        
        const mode = isView ? MODES.VIEW : MODES.EDIT;
        const title = isEdit ? 'Edit Post' : isView ? 'View Post' : 'Create New Post';

        return (
            <div>
                <div className="d-flex justify-content-between align-items-center mb-4">
                    <h1>{title}</h1>
                    <button 
                        className="btn btn-secondary"
                        onClick={this.handleBackToList}
                    >
                        Back to List
                    </button>
                </div>

                <div className="card">
                    <div className="card-body">
                        <FieldsetProvider module="posts" mode={mode}>
                            <FieldsetForm
                                mode={mode}
                                data={selectedPost}
                                onSubmit={isView ? undefined : this.handleSubmitPost}
                            />
                        </FieldsetProvider>
                    </div>
                </div>
            </div>
        );
    }

    render() {
        const { currentView, loading } = this.state;

        if (loading && currentView === 'list') {
            return (
                <div className="d-flex justify-content-center align-items-center" style={{ height: '400px' }}>
                    <div className="spinner-border" role="status">
                        <span className="sr-only">Loading...</span>
                    </div>
                </div>
            );
        }

        return (
            <div className="container-fluid">
                {currentView === 'list' ? this.renderListView() : this.renderFormView()}
            </div>
        );
    }
}

export default PostsPage;