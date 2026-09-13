/**
 * Ported from Seaglass frontend/src/registry/TimeSeriesChart.tsx; adapted to
 * the extended catalog's x/y point model, sizing, fill, and explicit domains.
 */
import { useId } from "react";
import {
  areaPath,
  scaleX,
  scaleXByDate,
  scaleYDomain,
  smoothPath,
  type Pt,
} from "./chart";
import { SERIES_COLORS } from "./types";

export interface LinePoint {
  x: string | number;
  y: number;
}

export interface LineSeries {
  name: string;
  points: LinePoint[];
}

export interface LineChartProps {
  series: LineSeries[];
  unit?: string;
  height?: number;
  yMin?: number;
  yMax?: number;
  fill?: boolean;
  xType?: "category" | "time";
}

type Role = "actual" | "forecast" | "target";

function roleOf(name: string): Role {
  const normalized = name.toLowerCase().trim();
  if (/\bactual\b/.test(normalized)) return "actual";
  if (/\btarget\b/.test(normalized)) return "target";
  if (/\bforecast\b/.test(normalized)) return "forecast";
  if (normalized.includes("target")) return "target";
  if (normalized.includes("forecast")) return "forecast";
  return "actual";
}

const W = 1000;
const H = 300;
const PAD_L = 52;
const PAD_R = 14;
const PAD_T = 14;
const PAD_B = 34;
const PLOT_W = W - PAD_L - PAD_R;
const PLOT_H = H - PAD_T - PAD_B;

function timeValue(value: string | number): number {
  if (typeof value === "number") return value;
  const parsed = Date.parse(value);
  return Number.isFinite(parsed) ? parsed : 0;
}

function labelFor(value: string | number, xType: "category" | "time"): string {
  if (xType === "category") return String(value);
  const parsed = new Date(timeValue(value));
  if (Number.isNaN(parsed.getTime())) return String(value);
  return parsed.toLocaleTimeString([], { hour: "numeric", minute: "2-digit" });
}

function compactLabel(label: string, maximum = 14): string {
  return label.length > maximum ? `${label.slice(0, maximum - 1)}…` : label;
}

export function LineChart({
  series,
  unit,
  height = 180,
  yMin,
  yMax,
  fill = false,
  xType = "category",
}: LineChartProps) {
  const gradientPrefix = useId();
  const usable = Array.isArray(series)
    ? series.filter((item) => Array.isArray(item.points) && item.points.length > 0)
    : [];
  if (usable.length === 0) {
    return <span className="extended-empty">No chart data</span>;
  }

  const values = usable.flatMap((item) => item.points.map((point) => point.y));
  let domainMin = yMin ?? Math.min(...values);
  let domainMax = yMax ?? Math.max(...values);
  if (domainMin === domainMax) {
    const padding = Math.abs(domainMin) * 0.1 || 1;
    domainMin -= padding;
    domainMax += padding;
  }

  const times = usable.flatMap((item) => item.points.map((point) => timeValue(point.x)));
  const minTime = Math.min(...times);
  const maxTime = Math.max(...times);
  const toPoints = (item: LineSeries): Pt[] =>
    item.points.map((point, index) => ({
      x:
        xType === "time"
          ? scaleXByDate(timeValue(point.x), minTime, maxTime, PAD_L, PLOT_W)
          : scaleX(index, item.points.length, PAD_L, PLOT_W),
      y: scaleYDomain(point.y, domainMin, domainMax, PAD_T, PLOT_H),
    }));
  const labels = usable.reduce(
    (longest, item) => (item.points.length > longest.points.length ? item : longest),
    usable[0],
  ).points;
  const labelStep = Math.max(1, Math.ceil(labels.length / 6));
  const yTicks = [0, 0.25, 0.5, 0.75, 1].map((ratio) => domainMax - ratio * (domainMax - domainMin));

  return (
    <figure className="extended-chart" style={{ minHeight: height }}>
      <svg
        viewBox={`0 0 ${W} ${H}`}
        preserveAspectRatio="none"
        role="img"
        aria-label="Line chart"
        style={{ height }}
      >
        <defs>
          {usable.map((_, index) => (
            <linearGradient key={index} id={`${gradientPrefix}-${index}`} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0" stopColor={SERIES_COLORS[index % SERIES_COLORS.length]} stopOpacity="0.2" />
              <stop offset="1" stopColor={SERIES_COLORS[index % SERIES_COLORS.length]} stopOpacity="0" />
            </linearGradient>
          ))}
        </defs>
        {yTicks.map((tick, index) => {
          const y = PAD_T + (index / 4) * PLOT_H;
          return (
            <g key={tick}>
              <line x1={PAD_L} x2={W - PAD_R} y1={y} y2={y} stroke="var(--grid)" strokeWidth="1" />
              <text x={PAD_L - 8} y={y + 4} textAnchor="end" className="extended-chart__axis">
                {Math.round(tick * 10) / 10}
              </text>
            </g>
          );
        })}
        {fill
          ? usable.map((item, index) => (
              <path
                key={`fill-${item.name}`}
                data-area=""
                d={areaPath(toPoints(item), PAD_T + PLOT_H)}
                fill={`url(#${gradientPrefix}-${index})`}
                stroke="none"
              />
            ))
          : null}
        {[...usable]
          .sort((left, right) => Number(roleOf(left.name) === "actual") - Number(roleOf(right.name) === "actual"))
          .map((item) => {
            const index = usable.indexOf(item);
            const role = roleOf(item.name);
            return (
              <path
                key={item.name}
                data-series={role}
                d={smoothPath(toPoints(item))}
                fill="none"
                stroke={role === "target" ? "var(--tline)" : SERIES_COLORS[index % SERIES_COLORS.length]}
                strokeWidth={role === "actual" ? 2.5 : 2}
                strokeOpacity={role === "forecast" ? 0.55 : 1}
                strokeDasharray={role === "actual" ? undefined : role === "forecast" ? "4 4" : "5 5"}
                strokeLinejoin="round"
                strokeLinecap="round"
                vectorEffect="non-scaling-stroke"
              />
            );
          })}
        {labels.map((point, index) =>
          index % labelStep === 0 || index === labels.length - 1 ? (
            <text
              key={`${point.x}-${index}`}
              x={xType === "time"
                ? scaleXByDate(timeValue(point.x), minTime, maxTime, PAD_L, PLOT_W)
                : scaleX(index, labels.length, PAD_L, PLOT_W)}
              y={H - 8}
              textAnchor={index === 0 ? "start" : index === labels.length - 1 ? "end" : "middle"}
              className="extended-chart__axis"
            >
              <title>{labelFor(point.x, xType)}</title>
              {compactLabel(labelFor(point.x, xType))}
            </text>
          ) : null,
        )}
      </svg>
      <figcaption className="extended-chart__legend">
        {usable.map((item, index) => (
          <span key={item.name} title={`${item.name}${unit ? ` (${unit})` : ""}`}>
            <i style={{ background: SERIES_COLORS[index % SERIES_COLORS.length] }} />
            <b>{item.name}{unit ? ` (${unit})` : ""}</b>
          </span>
        ))}
      </figcaption>
    </figure>
  );
}
