import React from "react";

interface Bit {
  value: number;
  name?: string;
  label?: string;
}

interface BitmaskViewProps {
  value?: number;
  bits?: Bit[];
  options?: Bit[];
}

// BitmaskView renders the names of the bits set in an integer bitmask value.
const BitmaskView: React.FC<BitmaskViewProps> = ({ value, bits, options }) => {
  const mask = value || 0;
  const list = bits || options || [];
  const names = list
    .filter((b) => b.value !== 0 && (mask & b.value) === b.value)
    .map((b) => b.name || b.label || String(b.value));
  if (names.length === 0) return <span className="text-muted">—</span>;
  return <span>{names.join(", ")}</span>;
};

export default BitmaskView;