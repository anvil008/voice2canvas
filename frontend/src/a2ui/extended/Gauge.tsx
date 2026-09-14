/**
 * Gauge component with bounded min/max values and semantic threshold tones
 * from the extended catalog.
 */
import { clamp, formatMetric, TONE_COLOR, type Tone } from "./types";

export interface GaugeThreshold {
  upTo: number;
  tone: Exclude<Tone, "neutral">;
}

export function toneForGauge(value: number, thresholds: GaugeThreshold[] = []): Tone {
  if (thresholds.length === 0) return "neutral";
  return thresholds.find((threshold) => value <= threshold.upTo)?.tone ?? thresholds[thresholds.length - 1].tone;
}

const SIZE = 132;
const RADIUS = 53;
const CIRCUMFERENCE = 2 * Math.PI * RADIUS;

export interface GaugeProps {
  label: string;
  value: number;
  min?: number;
  max: number;
  unit?: string;
  thresholds?: GaugeThreshold[];
}

export function Gauge({ label, value, min = 0, max, unit, thresholds = [] }: GaugeProps) {
  const safeValue = value != null && !isNaN(value) ? value : null;
  const safeMin = min != null && !isNaN(min) ? min : 0;
  const safeMax = max != null && !isNaN(max) ? max : null;

  const range = safeMax != null ? safeMax - safeMin : 0;
  const fraction = safeValue != null && range > 0 ? clamp((safeValue - safeMin) / range, 0, 1) : 0;
  const tone = safeValue != null ? toneForGauge(safeValue, thresholds) : "neutral";

  const displayValue = safeValue != null ? safeValue.toLocaleString() : "—";
  const displayMin = safeMin.toLocaleString();
  const displayMax = safeMax != null ? safeMax.toLocaleString() : "—";
  const metricTitle = safeValue != null ? formatMetric(safeValue, unit) : "—";

  return (
    <section className="extended-gauge" data-tone={tone}>
      <div className="extended-gauge__ring">
        <svg viewBox={`0 0 ${SIZE} ${SIZE}`} role="img" aria-label={`${label}: ${metricTitle}`}>
          <circle cx={SIZE / 2} cy={SIZE / 2} r={RADIUS} fill="none" stroke="var(--panel2)" strokeWidth="12" />
          <circle
            cx={SIZE / 2}
            cy={SIZE / 2}
            r={RADIUS}
            fill="none"
            stroke={TONE_COLOR[tone]}
            strokeWidth="12"
            strokeLinecap="round"
            strokeDasharray={`${CIRCUMFERENCE * fraction} ${CIRCUMFERENCE}`}
            transform={`rotate(-90 ${SIZE / 2} ${SIZE / 2})`}
            className="extended-gauge__value-ring"
          />
        </svg>
        <strong title={metricTitle}>{displayValue}<small>{unit}</small></strong>
      </div>
      <span className="extended-gauge__label" title={label}>{label}</span>
      <span className="extended-gauge__range">{displayMin} – {displayMax}{unit ? ` ${unit}` : ""}</span>
    </section>
  );
}
