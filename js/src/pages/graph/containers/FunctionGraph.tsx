import React, { Component } from "react";
import MarkdownRender from "@engine/fields/Markdown/controllers/MarkdownRender";
import "./canvas.css";

interface GraphFn {
    fn: (x: number, additionalParams?: any) => number;
    additionalParams?: any;
    color?: string;
    thick?: number;
    latex?: string; // rendered via KaTeX when present
    label?: string; // plain-text fallback (e.g. a user-typed expression)
}

interface FunctionGraphProps {
    fns: GraphFn[];
}

//todo
/**
 * add color picker
 * add quantiles logic
 * add default distributions
 * add axis descriptions
 * add CLT logic
 */
class FunctionGraph extends Component<FunctionGraphProps> {
    componentDidMount() {
        this.redraw();
    }

    // Re-plot whenever the function list changes (added/removed/edited) — the
    // canvas has no diffing of its own, so the simplest correct approach is to
    // clear and redraw everything each time.
    componentDidUpdate(prevProps: FunctionGraphProps) {
        if (prevProps.fns !== this.props.fns) this.redraw();
    }

    private redraw() {
        const canvas = document.querySelector<HTMLCanvasElement>("canvas#graphs");
        const ctx = canvas?.getContext("2d");
        if (!canvas || !ctx) return;

        const axes = {
            x0: 0.5 + 0.5 * canvas.width,
            y0: 0.5 + 0.5 * canvas.height,
            scale: 40,
            allowNegativeX: true,
            allowNegativeY: false,
        };

        ctx.clearRect(0, 0, canvas.width, canvas.height);
        this.drawAxes(ctx, axes);

        this.props.fns.forEach((fnParams) => {
            const { fn, additionalParams, color, thick } = fnParams;
            this.drawFunciton(ctx, axes, fn, color || "rgba(0, 0, 0, 255)", thick || 1, additionalParams || {});
        });
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
            <div id="2d" className="graph-2d">
                <div className="description">
                    {this.props.fns.map((fn, index) => {
                        const swatchColor = fn.color || "rgba(0, 0, 0, 1)";
                        // Formulas go through the same markdown pipeline (remark-math + rehype-katex) as posts, so KaTeX is loaded once.
                        const content = fn.latex ? (
                            <MarkdownRender className="inline-formula" value={`$${fn.latex}$`} />
                        ) : fn.label ? (
                            <code>{fn.label}</code>
                        ) : null;
                        if (!content) return null;
                        return (
                            <div className="new-line function-description" style={{ top: 40*(index+1) + "px" } } key={index}>
                                <div className="rect" style={{ backgroundColor: swatchColor }} />
                                {content}
                            {`\n`}</div>
                        )
                    })}
                </div>
                <canvas id="graphs" width="700" height="380"></canvas>
            </div>
        )
    }
}

export default FunctionGraph;
