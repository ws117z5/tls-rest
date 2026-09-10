import React from "react";

// UuidView renders a UUID value as the canonical 8-4-4-4-12 string, whatever
// shape it arrives in: an already-formatted string, 32 bare hex chars, a 16-byte
// array (pgx's raw form), a stringified byte array, or a delimited list of ints.

interface UuidViewProps {
  value?: any;
}

function dash(hex: string): string {
  const h = hex.toLowerCase();
  return `${h.slice(0, 8)}-${h.slice(8, 12)}-${h.slice(12, 16)}-${h.slice(16, 20)}-${h.slice(20, 32)}`;
}

function bytesToUuid(bytes: number[]): string | null {
  if (bytes.length !== 16 || bytes.some((n) => !Number.isInteger(n) || n < 0 || n > 255)) {
    return null;
  }
  return dash(bytes.map((n) => n.toString(16).padStart(2, "0")).join(""));
}

export function toUuid(value: any): string {
  if (value === null || value === undefined || value === "") return "";

  if (Array.isArray(value)) {
    return bytesToUuid(value.map(Number)) ?? value.join(",");
  }

  if (typeof value === "string") {
    const s = value.trim();
    if (!s) return "";
    // already canonical
    if (/^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$/.test(s)) {
      return s.toLowerCase();
    }
    // 32 bare hex chars
    const hex = s.replace(/[^0-9a-fA-F]/g, "");
    if (hex.length === 32 && /^[0-9a-fA-F]+$/.test(s.replace(/-/g, ""))) {
      return dash(hex);
    }
    // "[36,88,...]" or "36 88 ..." or "36,88,..."
    const parts = s.replace(/^\[|\]$/g, "").split(/[\s,]+/).filter(Boolean);
    if (parts.length === 16 && parts.every((p) => /^\d+$/.test(p))) {
      const u = bytesToUuid(parts.map(Number));
      if (u) return u;
    }
    return s;
  }

  return String(value);
}

const UuidView: React.FC<UuidViewProps> = ({ value }) => {
  const u = toUuid(value);
  return u ? <code>{u}</code> : <span className="text-muted">—</span>;
};

export default UuidView;
