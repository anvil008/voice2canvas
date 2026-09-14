import { A2uiMessageListSchema, type A2uiMessage } from "@a2ui/web_core/v0_9";
import type { ServerFrame } from "./protocol";
import mockA2uiJsonl from "./fixtures/mock-a2ui.jsonl?raw";

function parseFixture(): A2uiMessage[] {
  const records = mockA2uiJsonl
    .trim()
    .split("\n")
    .filter(Boolean)
    .map((line) => JSON.parse(line) as unknown);
  const result = A2uiMessageListSchema.safeParse(records);
  if (!result.success) {
    throw new Error(`Mock A2UI fixture is invalid: ${result.error.message}`);
  }
  return result.data;
}

function parseMessages(records: unknown[], label: string): A2uiMessage[] {
  const result = A2uiMessageListSchema.safeParse(records);
  if (!result.success) {
    throw new Error(`${label} is invalid: ${result.error.message}`);
  }
  return result.data;
}

export const mockA2uiMessages = parseFixture();

function messageSurfaceId(message: A2uiMessage): string {
  if ("createSurface" in message) return message.createSurface.surfaceId;
  if ("updateComponents" in message) return message.updateComponents.surfaceId;
  if ("updateDataModel" in message) return message.updateDataModel.surfaceId;
  return message.deleteSurface.surfaceId;
}

function messagesFor(surfaceId: string): A2uiMessage[] {
  return mockA2uiMessages.filter((message) => messageSurfaceId(message) === surfaceId);
}

const briefMessages = messagesFor("card_brief");
const taskMessages = messagesFor("card_tasks");
const briefCreation = briefMessages.filter(
  (message) => "createSurface" in message || "updateComponents" in message ||
    ("updateDataModel" in message && message.updateDataModel.path === "/"),
);
const briefUpdate = briefMessages.filter(
  (message) => "updateDataModel" in message && message.updateDataModel.path === "/summary",
);
const briefDelete = briefMessages.filter((message) => "deleteSurface" in message);
const taskCreation = taskMessages.filter((message) => !("deleteSurface" in message));

export const extendedWeatherCreationMessages = parseMessages([
  {
    version: "v0.9.1",
    createSurface: {
      surfaceId: "card_weather",
      catalogId: "https://voice2canvas.local/catalogs/extended/v1",
    },
  },
  {
    version: "v0.9.1",
    updateComponents: {
      surfaceId: "card_weather",
      components: [
        { id: "root", component: "Card", child: "weather_body" },
        { id: "weather_body", component: "Column", children: ["weather_title", "weather_stats", "weather_chart", "weather_facts"] },
        { id: "weather_title", component: "Text", variant: "h3", text: "Calgary forecast" },
        { id: "weather_stats", component: "StatGroup", children: ["weather_stat", "modernization_stat", "contributor_stat"], columns: 3 },
        {
          id: "weather_stat",
          component: "Stat",
          label: "Temperature now",
          value: { path: "/weather/temperature" },
          unit: "°C",
          delta: { path: "/weather/delta" },
          deltaLabel: "through this afternoon",
          tone: "positive",
          spark: { path: "/weather/spark" },
        },
        {
          id: "modernization_stat",
          component: "Stat",
          label: "Modernization candidates requiring architectural review",
          value: { path: "/weather/modernizations" },
          unit: "API calls",
          tone: "warning",
        },
        {
          id: "contributor_stat",
          component: "Stat",
          label: "High-risk contributors with unresolved ownership",
          value: { path: "/weather/contributors" },
          unit: "people",
          tone: "negative",
        },
        {
          id: "weather_chart",
          component: "LineChart",
          series: { path: "/weather/series" },
          unit: "°C",
          height: 190,
          fill: true,
          xType: "time",
        },
        { id: "weather_facts", component: "KeyValueList", rows: { path: "/weather/facts" } },
      ],
    },
  },
  {
    version: "v0.9.1",
    updateDataModel: {
      surfaceId: "card_weather",
      path: "/",
      value: {
        weather: {
          temperature: 22,
          delta: 4,
          spark: [13, 15, 18, 20, 22, 21],
          modernizations: 185,
          contributors: 12,
          series: [
            {
              name: "Forecast",
              points: [
                { x: "2026-08-01T06:00:00-06:00", y: 13 },
                { x: "2026-08-01T10:00:00-06:00", y: 17 },
                { x: "2026-08-01T14:00:00-06:00", y: 22 },
                { x: "2026-08-01T18:00:00-06:00", y: 20 },
                { x: "2026-08-01T22:00:00-06:00", y: 16 },
                { x: "2026-08-02T02:00:00-06:00", y: 12 },
              ],
            },
          ],
          facts: [
            { label: "Conditions", value: "Sunny with a very long unbroken advisory-reference-identifier-2026-08-01" },
            { label: "Wind", value: "14 km/h NW" },
            { label: "Sunset", value: "9:23 PM" },
          ],
        },
      },
    },
  },
], "Extended weather mock");

export const extendedComparisonCreationMessages = parseMessages([
  {
    version: "v0.9.1",
    createSurface: {
      surfaceId: "card_comparison",
      catalogId: "https://voice2canvas.local/catalogs/extended/v1",
    },
  },
  {
    version: "v0.9.1",
    updateComponents: {
      surfaceId: "card_comparison",
      components: [
        { id: "root", component: "Card", child: "comparison_body" },
        { id: "comparison_body", component: "Column", children: ["comparison_title", "comparison_chart"] },
        { id: "comparison_title", component: "Text", variant: "h3", text: "Weekend comparison" },
        {
          id: "comparison_chart",
          component: "BarChart",
          categories: { path: "/categories" },
          series: { path: "/series" },
          unit: "°C",
          height: 190,
        },
      ],
    },
  },
  {
    version: "v0.9.1",
    updateDataModel: {
      surfaceId: "card_comparison",
      path: "/",
      value: {
        categories: ["Calgary", "Banff", "Canmore"],
        series: [
          { name: "Saturday", values: [24, 19, 21] },
          { name: "Sunday", values: [21, 17, 20] },
        ],
      },
    },
  },
], "Extended comparison mock");

export const extendedGaugeCreationMessages = parseMessages([
  {
    version: "v0.9.1",
    createSurface: {
      surfaceId: "card_uv",
      catalogId: "https://voice2canvas.local/catalogs/extended/v1",
    },
  },
  {
    version: "v0.9.1",
    updateComponents: {
      surfaceId: "card_uv",
      components: [
        { id: "root", component: "Card", child: "uv_body" },
        { id: "uv_body", component: "Column", children: ["uv_title", "uv_gauge", "uv_badge", "uv_progress"] },
        { id: "uv_title", component: "Text", variant: "h3", text: "UV outlook" },
        {
          id: "uv_gauge",
          component: "Gauge",
          label: "Peak UV index",
          value: { path: "/uv" },
          max: 11,
          thresholds: [
            { upTo: 2, tone: "positive" },
            { upTo: 5, tone: "warning" },
            { upTo: 11, tone: "negative" },
          ],
        },
        { id: "uv_badge", component: "Badge", text: "Protection required", tone: "warning" },
        { id: "uv_progress", component: "ProgressBar", label: "Daylight elapsed", value: { path: "/daylight" }, tone: "neutral" },
      ],
    },
  },
  {
    version: "v0.9.1",
    updateDataModel: {
      surfaceId: "card_uv",
      path: "/",
      value: { uv: 6, daylight: 58 },
    },
  },
], "Extended gauge mock");

export const extendedWeatherUpdateMessages = parseMessages([
  {
    version: "v0.9.1",
    updateDataModel: {
      surfaceId: "card_weather",
      path: "/weather",
      value: {
        temperature: 25,
        delta: 7,
        spark: [13, 16, 20, 23, 25, 22],
        modernizations: 185,
        contributors: 12,
        series: [
          {
            name: "Forecast",
            points: [
              { x: "2026-08-01T06:00:00-06:00", y: 13 },
              { x: "2026-08-01T10:00:00-06:00", y: 18 },
              { x: "2026-08-01T14:00:00-06:00", y: 25 },
              { x: "2026-08-01T18:00:00-06:00", y: 22 },
              { x: "2026-08-01T22:00:00-06:00", y: 17 },
              { x: "2026-08-02T02:00:00-06:00", y: 13 },
            ],
          },
        ],
        facts: [
          { label: "Conditions", value: "Sunny with a very long unbroken advisory-reference-identifier-2026-08-01" },
          { label: "Wind", value: "17 km/h NW", tone: "warning" },
          { label: "Sunset", value: "9:23 PM" },
        ],
      },
    },
  },
], "Extended weather update mock");

/** A realistic backend-shaped script for ?mock=1; no WebSocket or microphone is used. */
export const mockScript: ReadonlyArray<{ atMs: number; frame: ServerFrame }> = [
  { atMs: 0, frame: { type: "ready", sessionId: "mock-session-001" } },
  { atMs: 250, frame: { type: "input_transcript", text: "Build a morning", final: false } },
  { atMs: 550, frame: { type: "input_transcript", text: "Build a morning briefing", final: true } },
  {
    atMs: 620,
    frame: {
      type: "task_status",
      taskId: "task_brief",
      surfaceId: "card_brief",
      status: "dispatched",
      detail: "Briefing request queued",
    },
  },
  {
    atMs: 650,
    frame: {
      type: "card_pending",
      surfaceId: "card_brief",
      taskId: "task_brief",
      title: "Morning briefing",
    },
  },
  {
    atMs: 700,
    frame: {
      type: "task_status",
      taskId: "task_brief",
      surfaceId: "card_brief",
      status: "researching",
      detail: "Gathering briefing context",
    },
  },
  {
    atMs: 900,
    frame: {
      type: "task_status",
      taskId: "task_brief",
      surfaceId: "card_brief",
      status: "generating",
      detail: "Creating briefing card",
    },
  },
  {
    atMs: 1_000,
    frame: { type: "a2ui", surfaceId: "card_brief", messages: briefCreation },
  },
  { atMs: 1_050, frame: { type: "layout", slots: [{ surfaceId: "card_brief", order: 0 }] } },
  {
    atMs: 1_300,
    frame: {
      type: "task_status",
      taskId: "task_brief",
      surfaceId: "card_brief",
      status: "rendered",
      detail: "Briefing is ready",
    },
  },
  { atMs: 1_450, frame: { type: "output_transcript", text: "I created your", final: false } },
  {
    atMs: 1_700,
    frame: {
      type: "output_transcript",
      text: "I created your morning briefing and suggested next steps.",
      final: true,
    },
  },
  {
    atMs: 1_850,
    frame: {
      type: "task_status",
      taskId: "task_steps",
      surfaceId: "card_tasks",
      status: "generating",
      detail: "Creating action list",
    },
  },
  {
    atMs: 2_100,
    frame: { type: "a2ui", surfaceId: "card_tasks", messages: taskCreation },
  },
  {
    atMs: 2_150,
    frame: {
      type: "layout",
      slots: [
        { surfaceId: "card_brief", order: 0 },
        { surfaceId: "card_tasks", order: 1 },
      ],
    },
  },
  {
    atMs: 2_350,
    frame: {
      type: "task_status",
      taskId: "task_steps",
      surfaceId: "card_tasks",
      status: "rendered",
      detail: "Suggested next steps are ready",
    },
  },
  {
    atMs: 2_700,
    frame: { type: "a2ui", surfaceId: "card_brief", messages: briefUpdate },
  },
  {
    atMs: 3_400,
    frame: { type: "a2ui", surfaceId: "card_brief", messages: briefDelete },
  },
  { atMs: 3_450, frame: { type: "card_removed", surfaceId: "card_brief" } },
  { atMs: 3_500, frame: { type: "layout", slots: [{ surfaceId: "card_tasks", order: 0 }] } },
  { atMs: 3_850, frame: { type: "input_transcript", text: "Show Calgary's weather and compare the weekend", final: true } },
  { atMs: 4_100, frame: { type: "a2ui", surfaceId: "card_weather", messages: extendedWeatherCreationMessages } },
  { atMs: 4_180, frame: { type: "a2ui", surfaceId: "card_comparison", messages: extendedComparisonCreationMessages } },
  { atMs: 4_260, frame: { type: "a2ui", surfaceId: "card_uv", messages: extendedGaugeCreationMessages } },
  {
    atMs: 4_320,
    frame: {
      type: "layout",
      slots: [
        { surfaceId: "card_tasks", order: 0 },
        { surfaceId: "card_weather", order: 1 },
        { surfaceId: "card_comparison", order: 2 },
        { surfaceId: "card_uv", order: 3 },
      ],
    },
  },
  { atMs: 4_600, frame: { type: "output_transcript", text: "Here is the 24-hour forecast, weekend comparison, and UV outlook.", final: true } },
  { atMs: 5_200, frame: { type: "a2ui", surfaceId: "card_weather", messages: extendedWeatherUpdateMessages } },
  {
    atMs: 5_250,
    frame: {
      type: "layout",
      slots: [
        { surfaceId: "card_weather", order: 0, span: 2 },
        { surfaceId: "card_uv", order: 1 },
        { surfaceId: "card_tasks", order: 2 },
        { surfaceId: "card_comparison", order: 3 },
      ],
    },
  },
];

export const MOCK_PLAYBACK_RATE = 2.5;

export function runMockScript(emit: (frame: ServerFrame) => void): () => void {
  const timers = mockScript.map(({ atMs, frame }) =>
    window.setTimeout(() => emit(frame), atMs * MOCK_PLAYBACK_RATE));
  return () => timers.forEach((timer) => window.clearTimeout(timer));
}
