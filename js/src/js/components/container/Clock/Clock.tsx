import React, { Component } from "react";
import "./clock.scss";

interface ClockProps {}
interface ClockState {}

export default class Clock extends Component<ClockProps, ClockState> {
  private width: number = 120;
  private height: number = 120;

  componentDidMount() {
    window.requestAnimationFrame(this.draw);
  }

  draw = () => {
    const canvas = document.querySelector("canvas#clock") as HTMLCanvasElement;
    const ctx = canvas.getContext("2d");

    if (!ctx) return;

    const width = canvas.width;
    const height = canvas.height;

    ctx.clearRect(0, 0, width, height); // Clear canvas
    ctx.save();

    const time = new Date();
    let hours = time.getHours();
    const minutes = time.getMinutes();
    const seconds = time.getSeconds();

    const x0 = 0.5 * width;
    const y0 = 0.5 * height;
    const r = width / 2 - 5;

    // Calculate angles
    hours = hours >= 12 ? hours - 12 : hours;
    const angleH = -Math.PI / 2 + (2 * Math.PI * hours) / 12;
    const angleM = -Math.PI / 2 + (2 * Math.PI * minutes) / 60;
    const angleS = -Math.PI / 2 + (2 * Math.PI * seconds) / 60;

    // Helper function to draw clock hands
    const drawLine = (
      ctx: CanvasRenderingContext2D,
      color: string,
      angle: number,
      lineWidth: number,
      radius: number
    ) => {
      ctx.strokeStyle = color;
      ctx.lineWidth = lineWidth;
      const x = x0 + radius * Math.cos(angle);
      const y = y0 + radius * Math.sin(angle);
      ctx.beginPath();
      ctx.moveTo(x0, y0);
      ctx.lineTo(x, y);
      ctx.closePath();
      ctx.stroke();
    };

    // Draw clock face
    ctx.beginPath();
    ctx.lineWidth = 5;
    ctx.strokeStyle = "rgba(20, 20, 235, 0.5)";
    ctx.arc(x0, y0, r, 0, Math.PI * 2);
    ctx.closePath();
    ctx.stroke();

    // Draw clock center
    ctx.beginPath();
    ctx.lineWidth = 1;
    ctx.strokeStyle = "rgba(0, 0, 0, 0.9)";
    ctx.fillStyle = "rgba(0,0,0,0.1)";
    ctx.arc(x0, y0, r + 3, 0, Math.PI * 2);
    ctx.closePath();
    ctx.fill();
    ctx.stroke();

    // Draw clock hands
    drawLine(ctx, "rgba(0, 55, 0, 0.8)", angleH, 3, r - 5); // Hour hand
    drawLine(ctx, "rgba(55, 0, 0, 0.8)", angleM, 2.5, r - 7); // Minute hand
    drawLine(ctx, "rgba(0, 0, 55, 0.8)", angleS, 2, r - 9); // Second hand

    window.requestAnimationFrame(this.draw);
  };

  render() {
    return (
      <div className="clock">
        <canvas width={this.width} height={this.height} id="clock"></canvas>
      </div>
    );
  }
}