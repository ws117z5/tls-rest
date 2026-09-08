export type Unit = "h" | "m" | "s";

// Which units a format string wants, in display order (biggest first).
export function unitsFromFormat(fmt?: string): Unit[] {
  const f = (fmt || "mm:ss");
  const units: Unit[] = [];
  if (/H/.test(f)) units.push("h");
  if (/m/.test(f)) units.push("m");
  if (/s/.test(f)) units.push("s");
  return units.length ? units : ["m", "s"];
}

export function toParts(totalSeconds: number, units: Unit[]): Record<Unit, number> {
  let rem = Math.max(0, Math.floor(totalSeconds || 0));
  const p: Record<Unit, number> = { h: 0, m: 0, s: 0 };
  if (units.includes("h")) { p.h = Math.floor(rem / 3600); rem %= 3600; }
  if (units.includes("m")) { p.m = Math.floor(rem / 60); rem %= 60; }
  if (units.includes("s")) { p.s = rem; }
  return p;
}

export function fromParts(p: Record<Unit, number>, units: Unit[]): number {
  let total = 0;
  if (units.includes("h")) total += (p.h || 0) * 3600;
  if (units.includes("m")) total += (p.m || 0) * 60;
  if (units.includes("s")) total += (p.s || 0);
  return total;
}

// Format total seconds as e.g. "01:30" (mm:ss) or "1:02:05" (HH:mm:ss).
export function formatDuration(totalSeconds: number, fmt?: string): string {
  const units = unitsFromFormat(fmt);
  const p = toParts(totalSeconds, units);
  const pad = (n: number) => String(n).padStart(2, "0");
  return units.map((u) => pad(p[u])).join(":");
}