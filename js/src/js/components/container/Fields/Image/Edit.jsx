import React, { Component } from "react";

//todo
class ImageEdit extends Component {
    onImage = (event) => {
        this.props.onChange(event);
    }

    render() {
        return (
            <input type="file" onChange={this.onImage} id="image" name="image" accept="image/*" />
            //this.props.value
        )
    }
}

export default ImageEdit;