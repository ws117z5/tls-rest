import React, { Component } from "react";
import ReactDOM from "react-dom";
import PageComponent from "../../controllers/PageComponent";
import ImageProcessing from "../container/ImageProcessing/ImageProcessing";

// Create a function to wrap up your component
class ImageProcessingPage extends PageComponent {
    static href = 'imageproc';
    static title = 'ImageProcessing';
    static isPage = true;

    render() {
        return (
            <div className="base">
                <ImageProcessing />
            </div>
        )
    }
}

export default ImageProcessingPage;