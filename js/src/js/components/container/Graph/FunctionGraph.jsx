import React, { Component } from "react";
import "./canvas.css";

import 'katex/dist/katex.min.css';
import { InlineMath, BlockMath } from 'react-katex';


//todo 
/**
 * add input form for formulas
 * add color picker
 * add quantiles logic
 * add default distributions
 * add axis descriptions
 * add CLT logic
 */
class FunctionGraph extends Component {
    constructor(props, state) {
        super(props, state);

        this.state = {
            fns: [],
            canvasDrawn: false
        }
    }

    componentWillReceiveProps(nextProps) {
        // This will erase any local state updates!
        // Do not do this.
        this.setState({ fns: nextProps.fns });
      }
    

    componentDidMount() {
        var canvas = document.querySelector("canvas#graphs");
        var axes = {};
        var ctx = canvas.getContext("2d");

        axes.x0 = 0.5 + 0.5*canvas.width;
        axes.y0 = 0.5 + 0.5*canvas.height;

        axes.scale = 40;
        axes.allowNegativeX = true;
        axes.allowNegativeY = false;

        this.drawAxes(ctx, axes);

        this.props.fns.forEach((fnParams) => {
            var {fn, additionalParams, color, thick} = fnParams;
            additionalParams = additionalParams ? additionalParams : {};
            color = color ? color : "rgba(0, 0, 0, 255)";
            thick = thick ? thick : 1;

            this.drawFunciton(ctx, axes, fn, color, thick, additionalParams);
        });
    }

    drawDescription = () => {

    }

    drawFunciton = (ctx, axes, func, color, thick, additionalParams) => {
        var xx, yy, dx = 4;

        var iMax = Math.round((ctx.canvas.width-axes.x0)/dx);
        var iMin = axes.allowNegativeX ? (-axes.x0/dx) : 0;

        ctx.beginPath();
        ctx.lineWidth = thick;
        ctx.strokeStyle = color;

        for(var i = iMin; i<=iMax; i++) {
            xx = dx+i;
            yy = axes.scale*func(xx/axes.scale, additionalParams);
            if(i == iMin) {
                ctx.moveTo(axes.x0+xx, axes.y0-yy);
            } else {
                ctx.lineTo(axes.x0+xx, axes.y0-yy);
            }

        } 
        ctx.stroke();
    }

    drawAxes = (ctx, axes) => {
        var xmin = axes.allowNegativeX ? 0 : axes.x0,
            ymin = axes.allowNegativeY ? ctx.canvas.height : axes.y0;

        ctx.beginPath();
        ctx.strokeStyle = "rgba(0, 0, 0, 0.5)";

        ctx.moveTo(xmin, axes.y0);
        ctx.lineTo(ctx.canvas.width, axes.y0);

        ctx.moveTo(axes.x0, 0);
        ctx.lineTo(axes.x0, ymin);

        ctx.stroke();
    }

    render() {
        return (
            <div id="2d">
                <div className="description">
                    {this.props.fns.map((fn, index) => {
                        //rgb(0,0,255);stroke-width:3;stroke:rgb(0,0,0)" />
                        if (fn.hasOwnProperty('latex')) {
                            return (
                                <div className="new-line function-description" style={{ top: 40*(index+1) + "px" } } key={index}>
                                    <div className="rect" style={ { backgroundColor: fn.hasOwnProperty('color') ? fn.color : "rgba(0, 0, 0, 1)" } } /> 
                                    <InlineMath math={fn.latex}> </InlineMath>
                                {`\n`}</div> 
                            )
                        }
                    })}
                </div>
                <canvas id="graphs" width="1200" height="800"></canvas>
            </div>
        )
    }
}

export default FunctionGraph;