import React from "react";

// One enhancer control, shaped from the model's own params list: a range slider or a select, per its "kind".
export interface ParamSpec {
  key: string;
  label: string;
  hint?: string;
  kind: "range" | "select";
  min?: number;
  max?: number;
  step?: number;
  default?: number;
  choices?: { value: number; label: string }[];
}

const RangeControl: React.FC<{ spec: ParamSpec; value: number; onChange: (v: number) => void; disabled?: boolean }> = ({
  spec,
  value,
  onChange,
  disabled,
}) => (
  <div className="col-6 col-md-4 mb-3">
    <div className="text-muted small text-uppercase mb-1" title={spec.hint}>
      {spec.label}
      {spec.kind === "range" && `: ${value}`}
    </div>
    {spec.kind === "select" ? (
      <select
        className="form-control"
        disabled={disabled}
        value={value}
        onChange={(e) => onChange(Number(e.target.value))}
      >
        {(spec.choices || []).map((c) => (
          <option key={c.value} value={c.value}>
            {c.label}
          </option>
        ))}
      </select>
    ) : (
      <input
        type="range"
        className="form-control"
        disabled={disabled}
        min={spec.min}
        max={spec.max}
        step={spec.step || 1}
        value={value}
        onChange={(e) => onChange(Number(e.target.value))}
      />
    )}
  </div>
);

export default RangeControl;
