import React, { Component, ChangeEvent } from "react";

//todo
class ImageEdit extends Component<ImageEditProps> {
    onImage = (event: ChangeEvent<HTMLInputElement>) => {
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