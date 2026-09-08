import React, { Component, ChangeEvent } from "react";
import { Unit, unitsFromFormat, toParts, fromParts } from "./util";

interface Props {
  id?: string;
  value?: number | string;   // total seconds (stored)
  format?: string;           // from field.options.format
  disabled?: boolean;
  onChange?: (value: number) => void;
}

const LABEL: Record<Unit, string> = { h: "Hours", m: "Min", s: "Sec" };

// Duration picker driven by a format ("HH:mm:ss" | "mm:ss" | "ss"). Renders one
// number input per unit; value is the total in seconds.
class TimeDurationEdit extends Component<Props> {
  render() {
    const { value, format, disabled, onChange } = this.props;
    const units = unitsFromFormat(format);
    const total = typeof value === "string" ? parseInt(value, 10) || 0 : (value || 0);
    const parts = toParts(total, units);

    const update = (u: Unit, e: ChangeEvent<HTMLInputElement>) => {
      const v = Math.max(0, parseInt(e.target.value, 10) || 0);
      const next = { ...parts, [u]: u === "h" ? v : Math.min(v, 59) };
      onChange?.(fromParts(next, units));
    };

    return (
      <div className="d-flex align-items-end" style={{ gap: 8 }}>
        {units.map((u, i) => (
          <React.Fragment key={u}>
            <div>
              <label className="form-label mb-0 small text-muted">{LABEL[u]}</label>
              <input
                type="number"
                min={0}
                max={u === "h" ? 99 : 59}
                disabled={disabled}
                className="form-control"
                style={{ width: 78 }}
                value={parts[u]}
                onChange={(e) => update(u, e)}
              />
            </div>
            {i < units.length - 1 && <span style={{ paddingBottom: 8 }}>:</span>}
          </React.Fragment>
        ))}
      </div>
    );
  }
}
export default TimeDurationEdit;