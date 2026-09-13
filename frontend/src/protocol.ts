/** Browser-facing messages defined by the repository's PROTOCOL.md. */

export type TaskState =
  | "dispatched"
  | "researching"
  | "generating"
  | "rendered"
  | "failed";

export interface ReadyFrame {
  type: "ready";
  sessionId: string;
}

export interface TranscriptFrame {
  type: "input_transcript" | "output_transcript";
  text: string;
  final: boolean;
}

export interface InterruptedFrame {
  type: "interrupted";
}

export interface A2uiFrame {
  type: "a2ui";
  surfaceId: string;
  /** Validated again by the renderer before it reaches MessageProcessor. */
  messages: unknown[];
}

export interface CardRemovedFrame {
  type: "card_removed";
  surfaceId: string;
}

export interface CardPendingFrame {
  type: "card_pending";
  surfaceId: string;
  taskId: string;
  title?: string;
}

export interface LayoutSlot {
  surfaceId: string;
  order: number;
  span?: 1 | 2;
}

export interface LayoutFrame {
  type: "layout";
  slots: LayoutSlot[];
}

export interface TaskStatusFrame {
  type: "task_status";
  taskId: string;
  surfaceId?: string;
  status: TaskState;
  detail?: string;
}

export interface ErrorFrame {
  type: "error";
  message: string;
  fatal: boolean;
}

export interface UsageFrame {
  type: "usage";
  inputTokens: number;
  outputTokens: number;
  source: "live" | "worker";
  at: string;
}

export type ServerFrame =
  | ReadyFrame
  | TranscriptFrame
  | InterruptedFrame
  | A2uiFrame
  | CardPendingFrame
  | CardRemovedFrame
  | LayoutFrame
  | TaskStatusFrame
  | UsageFrame
  | ErrorFrame;

export interface StartMessage {
  type: "start";
  /** Optional per-session key. It is never persisted by the reference client. */
  apiKey?: string;
}

export interface AudioEndMessage {
  type: "audio_end";
}

export interface StopMessage {
  type: "stop";
}

export interface ActionMessage {
  type: "action";
  surfaceId: string;
  name: string;
  sourceComponentId: string;
  context: Record<string, unknown>;
}

export type ClientFrame = StartMessage | AudioEndMessage | StopMessage | ActionMessage;

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isString(value: unknown): value is string {
  return typeof value === "string";
}

function isBoolean(value: unknown): value is boolean {
  return typeof value === "boolean";
}

function isTaskState(value: unknown): value is TaskState {
  return (
    value === "dispatched" ||
    value === "researching" ||
    value === "generating" ||
    value === "rendered" ||
    value === "failed"
  );
}

function isTokenCount(value: unknown): value is number {
  return Number.isInteger(value) && Number(value) >= 0;
}

function isIsoTimestamp(value: unknown): value is string {
  return isString(value) && Number.isFinite(Date.parse(value));
}

function parseSlots(value: unknown): LayoutSlot[] | undefined {
  if (!Array.isArray(value)) {
    return undefined;
  }

  const slots: LayoutSlot[] = [];
  for (const item of value) {
    if (!isRecord(item) || !isString(item.surfaceId) || typeof item.order !== "number") {
      return undefined;
    }
    const span = item.span;
    if ("span" in item && span !== 1 && span !== 2) {
      return undefined;
    }
    const slot: LayoutSlot = {
      surfaceId: item.surfaceId,
      order: item.order,
    };
    if (span === 1 || span === 2) slot.span = span;
    slots.push(slot);
  }
  return slots;
}

/**
 * Does lightweight wire validation before dispatching a text WebSocket frame.
 * A2UI message contents get their own strict schema validation in src/a2ui.
 */
export function parseServerFrame(value: unknown): ServerFrame | undefined {
  if (!isRecord(value) || !isString(value.type)) {
    return undefined;
  }

  switch (value.type) {
    case "ready":
      return isString(value.sessionId)
        ? { type: "ready", sessionId: value.sessionId }
        : undefined;
    case "input_transcript":
    case "output_transcript":
      return isString(value.text) && isBoolean(value.final)
        ? { type: value.type, text: value.text, final: value.final }
        : undefined;
    case "interrupted":
      return { type: "interrupted" };
    case "a2ui":
      return isString(value.surfaceId) && Array.isArray(value.messages)
        ? { type: "a2ui", surfaceId: value.surfaceId, messages: value.messages }
        : undefined;
    case "card_pending":
      return isString(value.surfaceId) && isString(value.taskId)
        ? {
            type: "card_pending",
            surfaceId: value.surfaceId,
            taskId: value.taskId,
            ...(isString(value.title) ? { title: value.title } : {}),
          }
        : undefined;
    case "card_removed":
      return isString(value.surfaceId)
        ? { type: "card_removed", surfaceId: value.surfaceId }
        : undefined;
    case "layout": {
      const slots = parseSlots(value.slots);
      return slots ? { type: "layout", slots } : undefined;
    }
    case "task_status":
      return isString(value.taskId) && isTaskState(value.status)
        ? {
            type: "task_status",
            taskId: value.taskId,
            status: value.status,
            ...(isString(value.surfaceId) ? { surfaceId: value.surfaceId } : {}),
            ...(isString(value.detail) ? { detail: value.detail } : {}),
          }
        : undefined;
    case "usage":
      return isTokenCount(value.inputTokens) &&
          isTokenCount(value.outputTokens) &&
          (value.source === "live" || value.source === "worker") &&
          isIsoTimestamp(value.at)
        ? {
            type: "usage",
            inputTokens: value.inputTokens,
            outputTokens: value.outputTokens,
            source: value.source,
            at: value.at,
          }
        : undefined;
    case "error":
      return isString(value.message) && isBoolean(value.fatal)
        ? { type: "error", message: value.message, fatal: value.fatal }
        : undefined;
    default:
      return undefined;
  }
}
