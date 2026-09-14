/**
 * SVG geometry and scaling utilities for extended catalog charts.
 */

export interface Pt {
  x: number;
  y: number;
}

/** Round to one decimal to keep path strings short and diff-stable. */
const r1 = (n: number): number => Math.round(n * 10) / 10;

/** Catmull-Rom spline converted to cubic Beziers, tension 1/6. */
export function smoothPath(pts: Pt[]): string {
  if (pts.length === 0) return "";
  if (pts.length === 1) return `M ${r1(pts[0].x)},${r1(pts[0].y)}`;
  if (pts.length === 2) {
    return `M ${r1(pts[0].x)},${r1(pts[0].y)} L ${r1(pts[1].x)},${r1(pts[1].y)}`;
  }

  let d = `M ${r1(pts[0].x)},${r1(pts[0].y)}`;
  for (let i = 0; i < pts.length - 1; i += 1) {
    const p0 = pts[i - 1] ?? pts[i];
    const p1 = pts[i];
    const p2 = pts[i + 1];
    const p3 = pts[i + 2] ?? p2;
    const c1x = p1.x + (p2.x - p0.x) / 6;
    const c1y = p1.y + (p2.y - p0.y) / 6;
    const c2x = p2.x - (p3.x - p1.x) / 6;
    const c2y = p2.y - (p3.y - p1.y) / 6;
    d += ` C ${r1(c1x)},${r1(c1y)} ${r1(c2x)},${r1(c2y)} ${r1(p2.x)},${r1(p2.y)}`;
  }
  return d;
}

/** The smoothed curve, dropped to a baseline at both ends and closed. */
export function areaPath(pts: Pt[], baselineY: number): string {
  if (pts.length === 0) return "";
  const top = smoothPath(pts);
  const lineTo = `L${top.slice(1)}`;
  const first = pts[0];
  const last = pts[pts.length - 1];
  return `M ${r1(first.x)},${r1(baselineY)} ${lineTo} L ${r1(last.x)},${r1(baselineY)} Z`;
}

/** Evenly spread index i of n points across the plot width. */
export function scaleX(i: number, n: number, padL: number, plotW: number): number {
  if (n <= 1) return padL;
  return padL + (i / (n - 1)) * plotW;
}

/** Map a timestamp against a shared time domain onto the plot width. */
export function scaleXByDate(
  t: number,
  minT: number,
  maxT: number,
  padL: number,
  plotW: number,
): number {
  if (maxT === minT) return padL;
  return padL + ((t - minT) / (maxT - minT)) * plotW;
}

/** Map a value against a fixed 0..max domain, inverted for SVG's y-down axis. */
export function scaleY(v: number, max: number, padT: number, plotH: number): number {
  if (max <= 0) return padT + plotH;
  return padT + (1 - v / max) * plotH;
}

/** Map a value against an arbitrary domain, inverted for SVG's y-down axis. */
export function scaleYDomain(
  value: number,
  min: number,
  max: number,
  padT: number,
  plotH: number,
): number {
  if (max === min) return padT + plotH / 2;
  return padT + (1 - (value - min) / (max - min)) * plotH;
}
