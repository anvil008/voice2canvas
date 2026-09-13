export type Tone = "neutral" | "positive" | "negative" | "warning";

export const TONE_COLOR: Record<Tone, string> = {
  neutral: "var(--tx)",
  positive: "var(--grn)",
  negative: "var(--red)",
  warning: "var(--pace)",
};

export const SERIES_COLORS = [
  "var(--chart1)",
  "var(--chart2)",
  "var(--chart3)",
  "var(--chart4)",
] as const;

export function clamp(value: number, min: number, max: number): number {
  return Math.min(Math.max(value, min), max);
}

export function formatMetric(value: string | number, unit?: string): string {
  const display = typeof value === "number" ? value.toLocaleString() : value;
  return unit ? `${display} ${unit}` : display;
}
