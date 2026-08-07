import React, { Component, ChangeEvent } from "react";
import "./image.processing.css";
import { ImageEdit, SelectEdit } from "../Fields";

//todo move to typescript to speed up the pixel manipulation process
export default class ImageProcessing extends Component {
    canvas;
    ctx;
    draggingMouse = false;
    originalImageData;

    constructor(props, state) {
        super(props, state);

        this.state = {
            canvasDrawn: false
        }
    }

    onMouseDown = (event) => {
        this.draggingMouse = true;

        console.log('mouse down')
    }

    onMouseUp = (event) => {
        this.draggingMouse = false;

        console.log('mouse up')
    }
    

    onLoad = (event: ChangeEvent<HTMLInputElement>) => {
        const files = event.target.files;
        if (!files || files.length === 0) return;

        const file = files[0];
        var _URL = window.URL || window.webkitURL;

        if (file) {
            
            let img = new Image();
            img.onload = () => {
                //console.log(this.width + " " + this.height);
                var {canvas, ctx } = this;

                if (canvas && ctx) {
                    // 'img.width' and 'img.height' are explicitly typed
                    canvas.width = img.width;
                    canvas.height = img.height;

                    ctx.drawImage(img, 0, 0, img.width, img.height);
                    this.originalImageData = ctx.getImageData(0, 0, canvas.width, canvas.height);
                }
            };
            img.onerror = () => {
                console.log("Not a valid file: " + file.type);
            };

            img.src = _URL.createObjectURL(file);
        }
    }

    componentDidMount() {
        this.canvas = document.querySelector("canvas#im_proc_canvas");
        this.ctx = this.canvas.getContext("2d");
    }

    evaluateFilter = (filter, that) => {
        //sepia
        if (typeof that === "undefined" || typeof that.ctx === "undefined") {
            return;
        }

        let ctx = that.ctx;
        let canvas = that.canvas;
        const imageData = this.originalImageData;
        const data = imageData.data;
        const dataCopy = [...data];

        // for (var i = 0; i < data.length; i += 4) {
        //     data[i] = (data[i] * .393) + (data[i + 1] *.769) + (data[i + 2] * .189)
        //     data[i + 1] = (data[i] * .349) + (data[i + 1] *.686) + (data[i + 2] * .168)
        //     data[i + 2] = (data[i] * .272) + (data[i + 1] *.534) + (data[i + 2] * .131)
            
        //     // data[i]         // red
        //     // data[i + 1]     // green
        //     // data[i + 2]     // blue
        // }

        imageData.getPixelAt = (x, y) => {
            var i = x*4 + canvas.width*y*4;

            return new Pixel(dataCopy[i], dataCopy[i+1], dataCopy[i+2]);
        }

        imageData.setPixelAt = (x, y, rgb) => {
            var i = x*4 + canvas.width*y*4;

            data[i] = rgb.r | 0;
            data[i+1] = rgb.g | 0;
            data[i+2] = rgb.b | 0;
            //data[i+3] = rgb.a;
        }

        //todo address the i=1 problem, fix for 
        if(typeof filters[filter].kernel !== 'undefined') {

            const kernel = filters[filter].kernel;
            for (var x = 1; x < canvas.width; x++) {
                for (var y = 1; y < canvas.height; y++) {
                    
                    //todo add function based mapping (Gaussian distr)
                    //todo add scales for kernel matrices
                    var pixel = new Pixel();
                    for(var i=-1, ii=0; i <= 1; i++, ii++) {
                        for(var j=-1, jj=0; j <= 1; j++, jj++) {
                            
                            var rgb = imageData.getPixelAt(x+i, y+j).mult(kernel[ii][jj]);
                
                            pixel.add(rgb);
                        }
                    }
    
                    imageData.setPixelAt(x, y, pixel);
                }
            }
        }

        
        ctx.putImageData(imageData, 0, 0);
    }



    render() {

        //var params = [this.ctx, this.canvas];

        return (
            <div id="im_proc" onMouseDown={this.onMouseDown} onMouseUp={this.onMouseUp}>
                <ImageEdit onChange={this.onLoad}/>
                <SelectEdit options={filters} onChange={this.evaluateFilter} params={this}/> 

                <canvas id="im_proc_canvas"></canvas>
            </div>
        )
    }
}


class Pixel {
    r = 0;
    g = 0;
    b = 0;
    a = 255;
    
    constructor(r = 0, g = 0, b = 0, a = 255) {
        this.r = r;
        this.g = g;
        this.b = b;

        this.a = a
    }

    mult = (n) => {
        this.r = this.r*n;
        this.g = this.g*n;
        this.b = this.b*n;

        return this;
    }

    add = (rgb) => {
        
        this.r += rgb.r;// * (rgb.a / 255);
        this.g += rgb.g;// * (rgb.a / 255);
        this.b += rgb.b;// * (rgb.a / 255);

        return this;
    }
}


const filters = [
    {
        name: "Original"
    },
    {
        name: "Box Blur",
        kernel: [
            [1/9, 1/9, 1/9],
            [1/9, 1/9, 1/9],
            [1/9, 1/9, 1/9],
        ]
    },
    {
        name: "Gaussian Blur",
        kernel: [
            [1/16, 1/8, 1/16],
            [1/8, 1/4, 1/8],
            [1/16, 1/8, 1/16],
        ]
    },
    {
        name: "Edge Detection",
        kernel: [
            [-1, -1, -1],
            [-1, 8, -1],
            [-1, -1, -1],
        ]
    },
    {
        name: "Sharpen",
        kernel: [
            [0, -1, 0],
            [-1, 5, -1],
            [0, -1, 0],
        ]
    }
]