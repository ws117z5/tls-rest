import React, { Component } from "react";
import ReactDOM from "react-dom";
import PageComponent from "@controllers/PageComponent";
import ImageProcessing from "@containers/ImageProcessing/ImageProcessing";

// Create a function to wrap up your component
class ImageProcessingPage extends PageComponent {
    protected href = 'imageproc';
    protected title = 'ImageProcessing';
    protected isPage = true;

    render() {
        return (
            <div className="base">
                <ImageProcessing />
            </div>
        )
    }
}

export default ImageProcessingPage;