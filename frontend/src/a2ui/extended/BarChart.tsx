/**
 * Ported from Seaglass frontend/src/registry/CategoryBarChart.tsx; extended
 * minimally for multiple series, grouped/stacked bars, and both orientations.
 */
import { SERIES_COLORS } from "./types";

export interface BarSeries {
  name: string;
  values: number[];
}

export interface BarChartProps {
  categories: string[];
  series: BarSeries[];
  unit?: string;
  height?: number;
  stacked?: boolean;
  horizontal?: boolean;
}

const W = 1000;
const H = 300;
const PAD_L = 52;
const PAD_R = 16;
const PAD_T = 12;
const PAD_B = 44;
const PLOT_W = W - PAD_L - PAD_R;
const PLOT_H = H - PAD_T - PAD_B;

function compactLabel(label: string, maximum = 14): string {
  return label.length > maximum ? `${label.slice(0, maximum - 1)}…` : label;
}

export function calculateBarMax(categories: string[], series: BarSeries[], stacked: boolean): number {
  if (stacked) {
    return Math.max(
      ...categories.map((_, index) => series.reduce((sum, item) => sum + Math.max(0, item.values[index] ?? 0), 0)),
      0,
    ) || 1;
  }
  return Math.max(...series.flatMap((item) => item.values), 0) || 1;
}

export function scaleBarHeight(value: number, max: number, plotHeight: number): number {
  return max > 0 ? (Math.max(0, value) / max) * plotHeight : 0;
}

function HorizontalBars({ categories, series, unit, stacked }: Omit<BarChartProps, "height" | "horizontal">) {
  const max = calculateBarMax(categories, series, Boolean(stacked));
  return (
    <div className="extended-bars-horizontal">
      {categories.map((category, categoryIndex) => (
        <div className="extended-bars-horizontal__row" key={category}>
          <span className="extended-bars-horizontal__label" title={category}>{category}</span>
          <div className={`extended-bars-horizontal__tracks${stacked ? " is-stacked" : ""}`}>
            {series.map((item, seriesIndex) => {
              const value = Math.max(0, item.values[categoryIndex] ?? 0);
              const width = value === 0 ? 0 : Math.max(6, (value / max) * 100);
              return (
                <div
                  className="extended-bars-horizontal__fill"
                  data-bar-fill=""
                  key={item.name}
                  title={`${item.name}: ${value.toLocaleString()}${unit ? ` ${unit}` : ""}`}
                  style={{ background: SERIES_COLORS[seriesIndex % SERIES_COLORS.length], width: `${width}%` }}
                />
              );
            })}
          </div>
          <span className="extended-bars-horizontal__value">
            {series.length === 1 ? (series[0].values[categoryIndex] ?? 0).toLocaleString() : ""}
          </span>
        </div>
      ))}
      <ChartLegend series={series} unit={unit} />
    </div>
  );
}

function ChartLegend({ series, unit }: { series: BarSeries[]; unit?: string }) {
  return (
    <div className="extended-chart__legend">
      {series.map((item, index) => (
        <span key={item.name} title={`${item.name}${unit ? ` (${unit})` : ""}`}>
          <i style={{ background: SERIES_COLORS[index % SERIES_COLORS.length] }} />
          <b>{item.name}{unit ? ` (${unit})` : ""}</b>
        </span>
      ))}
    </div>
  );
}

export function BarChart({
  categories,
  series,
  unit,
  height = 180,
  stacked = false,
  horizontal = false,
}: BarChartProps) {
  const usableSeries = Array.isArray(series) ? series.filter((item) => Array.isArray(item.values)) : [];
  if (!Array.isArray(categories) || categories.length === 0 || usableSeries.length === 0) {
    return <span className="extended-empty">No chart data</span>;
  }
  if (horizontal) {
    return <HorizontalBars categories={categories} series={usableSeries} unit={unit} stacked={stacked} />;
  }

  const max = calculateBarMax(categories, usableSeries, stacked);
  const groupWidth = PLOT_W / categories.length;
  const occupiedWidth = Math.min(groupWidth * 0.72, 130);
  const barWidth = stacked ? occupiedWidth : occupiedWidth / usableSeries.length;
  return (
    <figure className="extended-chart" style={{ minHeight: height }}>
      <svg
        viewBox={`0 0 ${W} ${H}`}
        preserveAspectRatio="none"
        role="img"
        aria-label="Bar chart"
        style={{ height }}
      >
        {[0, 0.25, 0.5, 0.75, 1].map((ratio) => {
          const y = PAD_T + ratio * PLOT_H;
          const tick = max * (1 - ratio);
          return (
            <g key={ratio}>
              <line x1={PAD_L} x2={W - PAD_R} y1={y} y2={y} stroke="var(--grid)" />
              <text x={PAD_L - 8} y={y + 4} textAnchor="end" className="extended-chart__axis">
                {Math.round(tick * 10) / 10}
              </text>
            </g>
          );
        })}
        {categories.flatMap((category, categoryIndex) => {
          let stackValue = 0;
          const groupStart = PAD_L + categoryIndex * groupWidth + (groupWidth - occupiedWidth) / 2;
          return usableSeries.map((item, seriesIndex) => {
            const value = Math.max(0, item.values[categoryIndex] ?? 0);
            const barHeight = scaleBarHeight(value, max, PLOT_H);
            const y = stacked
              ? PAD_T + PLOT_H - ((stackValue + value) / max) * PLOT_H
              : PAD_T + PLOT_H - barHeight;
            const x = stacked ? groupStart : groupStart + seriesIndex * barWidth;
            stackValue += value;
            return (
              <rect
                key={`${category}-${item.name}`}
                data-bar-fill=""
                x={x}
                y={y}
                width={Math.max(1, barWidth - (stacked ? 0 : 3))}
                height={barHeight}
                rx="5"
                fill={SERIES_COLORS[seriesIndex % SERIES_COLORS.length]}
              >
                <title>{`${item.name}: ${value.toLocaleString()}${unit ? ` ${unit}` : ""}`}</title>
              </rect>
            );
          });
        })}
        {categories.map((category, index) => (
          <text
            key={category}
            x={PAD_L + index * groupWidth + groupWidth / 2}
            y={H - 12}
            textAnchor="middle"
            className="extended-chart__axis"
          >
            <title>{category}</title>
            {compactLabel(category)}
          </text>
        ))}
      </svg>
      <ChartLegend series={usableSeries} unit={unit} />
    </figure>
  );
}
