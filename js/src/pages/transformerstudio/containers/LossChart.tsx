import React from "react";

const W = 600;
const H = 200;
const PAD = 28;

// Transformer Studio loss history as a dependency-free SVG line.
const LossChart: React.FC<{ values: number[] }> = ({ values }) => {
  if (values.length < 2) return <div className="text-muted small">—</div>;
  const min = Math.min(...values);
  const max = Math.max(...values);
  const span = max - min || 1;
  const x = (i: number) => PAD + (i / (values.length - 1)) * (W - PAD * 2);
  const y = (v: number) => H - PAD - ((v - min) / span) * (H - PAD * 2);
  const points = values.map((v, i) => `${x(i).toFixed(1)},${y(v).toFixed(1)}`).join(" ");
  return (
    <svg viewBox={`0 0 ${W} ${H}`} style={{ width: "100%", height: 200, color: "inherit" }}>
      <line x1={PAD} y1={H - PAD} x2={W - PAD} y2={H - PAD} stroke="currentColor" opacity={0.25} />
      <line x1={PAD} y1={PAD} x2={PAD} y2={H - PAD} stroke="currentColor" opacity={0.25} />
      <text x={PAD - 4} y={PAD + 4} textAnchor="end" fontSize={10} fill="currentColor" opacity={0.6}>{max.toFixed(3)}</text>
      <text x={PAD - 4} y={H - PAD} textAnchor="end" fontSize={10} fill="currentColor" opacity={0.6}>{min.toFixed(3)}</text>
      <polyline points={points} fill="none" stroke="#0d6efd" strokeWidth={2} strokeLinejoin="round" />
    </svg>
  );
};

export default LossChart;
