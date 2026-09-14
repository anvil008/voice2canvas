/**
 * Stat metric component displaying label, value, delta, tone, and optional sparkline.
 */
import { scaleX, scaleYDomain, smoothPath } from "./chart";
import { TONE_COLOR, type Tone } from "./types";

export interface StatProps {
  label: string;
  value: string | number;
  unit?: string;
  delta?: number;
  deltaLabel?: string;
  tone?: Tone;
  spark?: number[];
}

export function Stat({
  label,
  value,
  unit,
  delta,
  deltaLabel,
  tone = "neutral",
  spark,
}: StatProps) {
  const values = Array.isArray(spark) ? spark.filter(Number.isFinite) : [];
  const min = values.length ? Math.min(...values) : 0;
  const max = values.length ? Math.max(...values) : 0;
  const points = values.map((item, index) => ({
    x: scaleX(index, values.length, 2, 116),
    y: scaleYDomain(item, min, max, 3, 26),
  }));

  return (
    <section className="extended-stat" data-tone={tone}>
      <span className="extended-stat__label">{label}</span>
      <div className="extended-stat__main">
        <span className="extended-stat__reading">
          <span className="extended-stat__value" style={{ color: TONE_COLOR[tone] }}>{value}</span>
          {unit ? <span className="extended-stat__unit">{unit}</span> : null}
        </span>
        {points.length > 0 ? (
          <svg
            className="extended-stat__spark"
            viewBox="0 0 120 32"
            role="img"
            aria-label={`${label} trend`}
          >
            <path
              d={smoothPath(points)}
              fill="none"
              stroke={TONE_COLOR[tone]}
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
              vectorEffect="non-scaling-stroke"
            />
          </svg>
        ) : null}
      </div>
      {delta !== undefined || deltaLabel ? (
        <span className="extended-stat__delta">
          {delta !== undefined ? `${delta > 0 ? "+" : ""}${delta.toLocaleString()}` : null}
          {delta !== undefined && deltaLabel ? " " : null}
          {deltaLabel}
        </span>
      ) : null}
    </section>
  );
}
