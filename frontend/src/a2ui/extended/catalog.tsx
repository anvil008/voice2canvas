import { createComponentImplementation, type ReactComponentImplementation } from "@a2ui/react/v0_9";
import { z } from "zod";
import { Badge } from "./Badge";
import { BarChart, type BarChartProps } from "./BarChart";
import { DataTable } from "./DataTable";
import { ExtendedWarning } from "./ExtendedWarning";
import { Gauge, type GaugeProps } from "./Gauge";
import { KeyValueList, type KeyValueRow } from "./KeyValueList";
import { LineChart, type LineChartProps } from "./LineChart";
import { ProgressBar } from "./ProgressBar";
import { Stat, type StatProps } from "./Stat";
import { StatGroup } from "./StatGroup";
import type { Tone } from "./types";

export const EXTENDED_CATALOG_ID = "https://voice2canvas.local/catalogs/extended/v1";
export const LEGACY_EXTENDED_CATALOG_ID = "https://v2ui.local/catalogs/extended/v1";

const binding = z.object({ path: z.string().regex(/^\//) }).strict();
const dynamicString = z.union([z.string(), binding]);
const dynamicNumber = z.union([z.number(), binding]);
const dynamicStringOrNumber = z.union([z.string(), z.number(), binding]);
const tone = z.enum(["neutral", "positive", "negative", "warning"]);
const point = z.object({ x: z.union([z.string(), z.number()]), y: z.number() }).strict();
const lineSeries = z.object({ name: z.string(), points: z.array(point) }).strict();
const barSeries = z.object({ name: z.string(), values: z.array(z.number()) }).strict();

export const extendedComponentSchemas = {
  Stat: z.object({
    label: dynamicString,
    value: dynamicStringOrNumber,
    unit: dynamicString.optional(),
    delta: dynamicNumber.optional(),
    deltaLabel: dynamicString.optional(),
    tone: tone.optional(),
    spark: z.union([z.array(z.number()), binding]).optional(),
  }).strict(),
  StatGroup: z.object({
    children: z.array(z.string()).min(1),
    columns: z.number().int().min(1).max(4).optional(),
  }).strict(),
  LineChart: z.object({
    series: z.union([z.array(lineSeries).min(1), binding]),
    unit: dynamicString.optional(),
    height: z.number().int().min(80).max(480).optional(),
    yMin: z.number().optional(),
    yMax: z.number().optional(),
    fill: z.boolean().optional(),
    xType: z.enum(["category", "time"]).optional(),
  }).strict(),
  BarChart: z.object({
    categories: z.union([z.array(z.string()), binding]),
    series: z.union([z.array(barSeries).min(1), binding]),
    unit: dynamicString.optional(),
    height: z.number().int().min(80).max(480).optional(),
    stacked: z.boolean().optional(),
    horizontal: z.boolean().optional(),
  }).strict(),
  Gauge: z.object({
    label: dynamicString,
    value: dynamicNumber,
    min: z.number().optional(),
    max: z.number(),
    unit: dynamicString.optional(),
    thresholds: z.array(z.object({
      upTo: z.number(),
      tone: z.enum(["positive", "warning", "negative"]),
    }).strict()).optional(),
  }).strict(),
  ProgressBar: z.object({
    label: dynamicString.optional(),
    value: dynamicNumber,
    tone: tone.optional(),
  }).strict(),
  KeyValueList: z.object({
    rows: z.union([
      z.array(z.object({
        label: z.string(),
        value: z.union([z.string(), z.number()]),
        tone: tone.optional(),
      }).strict()).min(1),
      binding,
    ]),
  }).strict(),
  Badge: z.object({ text: dynamicString, tone: tone.optional() }).strict(),
  DataTable: z.object({
    columns: z.array(z.string()).min(1),
    rows: z.union([z.array(z.array(z.union([z.string(), z.number()]))), binding]),
    align: z.array(z.enum(["left", "right"])).optional(),
  }).strict(),
} as const;

export type ExtendedComponentName = keyof typeof extendedComponentSchemas;

const StatImplementation = createComponentImplementation(
  { name: "Stat", schema: extendedComponentSchemas.Stat },
  ({ props }) => <Stat {...(props as StatProps)} />,
);

const StatGroupImplementation = createComponentImplementation(
  { name: "StatGroup", schema: extendedComponentSchemas.StatGroup },
  ({ props, buildChild }) => (
    <StatGroup columns={props.columns}>
      {props.children.map((id: string) => buildChild(id))}
    </StatGroup>
  ),
);

const LineChartImplementation = createComponentImplementation(
  { name: "LineChart", schema: extendedComponentSchemas.LineChart },
  ({ props }) => <LineChart {...(props as LineChartProps)} />,
);

const BarChartImplementation = createComponentImplementation(
  { name: "BarChart", schema: extendedComponentSchemas.BarChart },
  ({ props }) => <BarChart {...(props as BarChartProps)} />,
);

const GaugeImplementation = createComponentImplementation(
  { name: "Gauge", schema: extendedComponentSchemas.Gauge },
  ({ props }) => <Gauge {...(props as GaugeProps)} />,
);

const ProgressBarImplementation = createComponentImplementation(
  { name: "ProgressBar", schema: extendedComponentSchemas.ProgressBar },
  ({ props }) => (
    <ProgressBar label={props.label} value={props.value} tone={props.tone as Tone | undefined} />
  ),
);

const KeyValueListImplementation = createComponentImplementation(
  { name: "KeyValueList", schema: extendedComponentSchemas.KeyValueList },
  ({ props }) => <KeyValueList rows={props.rows as KeyValueRow[]} />,
);

const BadgeImplementation = createComponentImplementation(
  { name: "Badge", schema: extendedComponentSchemas.Badge },
  ({ props }) => <Badge text={props.text} tone={props.tone as Tone | undefined} />,
);

const DataTableImplementation = createComponentImplementation(
  { name: "DataTable", schema: extendedComponentSchemas.DataTable },
  ({ props }) => <DataTable {...props} />,
);

export const EXTENDED_WARNING_COMPONENT = "ExtendedWarning";
const warningSchema = z.object({ message: z.string() }).strict();
const WarningImplementation = createComponentImplementation(
  { name: EXTENDED_WARNING_COMPONENT, schema: warningSchema },
  ({ props }) => <ExtendedWarning message={props.message} />,
);

export const extendedComponentImplementations: ReactComponentImplementation[] = [
  StatImplementation,
  StatGroupImplementation,
  LineChartImplementation,
  BarChartImplementation,
  GaugeImplementation,
  ProgressBarImplementation,
  KeyValueListImplementation,
  BadgeImplementation,
  DataTableImplementation,
  WarningImplementation,
];

export function validateExtendedComponent(
  component: string,
  properties: Record<string, unknown>,
): string | undefined {
  const schema = extendedComponentSchemas[component as ExtendedComponentName];
  if (!schema) return `Unsupported extended component: ${component}`;
  const result = schema.safeParse(properties);
  if (result.success) return undefined;
  const issue = result.error.issues[0];
  const location = issue.path.length > 0 ? issue.path.join(".") : "props";
  return `Invalid ${component} ${location}`;
}
