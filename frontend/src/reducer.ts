import type { LayoutSlot, ServerFrame, TaskState } from "./protocol";

export interface TranscriptLine {
  id: number;
  direction: "input" | "output";
  final: boolean;
  text: string;
}

export interface TaskItem {
  taskId: string;
  surfaceId?: string;
  status: TaskState;
  detail?: string;
}

export interface DashboardStatus {
  message: string;
  tone: "idle" | "ready" | "working" | "error";
}

export interface PendingCard {
  surfaceId: string;
  taskId: string;
  title?: string;
}

export interface DashboardState {
  canvasGeneration: number;
  nextTranscriptId: number;
  pendingCards: PendingCard[];
  removedSurfaceIds: string[];
  sessionId?: string;
  slots: LayoutSlot[];
  status: DashboardStatus;
  tasks: Record<string, TaskItem>;
  transcripts: TranscriptLine[];
}

export const initialDashboardState: DashboardState = {
  canvasGeneration: 0,
  nextTranscriptId: 1,
  pendingCards: [],
  removedSurfaceIds: [],
  slots: [],
  status: { message: "Waiting to connect", tone: "idle" },
  tasks: {},
  transcripts: [],
};

export type DashboardAction =
  | { type: "server"; frame: ServerFrame }
  | { type: "local_status"; message: string; tone?: DashboardStatus["tone"] };

function replaceTranscript(
  state: DashboardState,
  direction: TranscriptLine["direction"],
  text: string,
  final: boolean,
): Pick<DashboardState, "nextTranscriptId" | "transcripts"> {
  const transcriptIndex = [...state.transcripts]
    .reverse()
    .findIndex((line) => line.direction === direction && !line.final);
  const actualIndex = transcriptIndex === -1 ? -1 : state.transcripts.length - 1 - transcriptIndex;

  if (actualIndex !== -1) {
    const transcripts = state.transcripts.map((line, index) =>
      index === actualIndex ? { ...line, text, final } : line,
    );
    return { transcripts, nextTranscriptId: state.nextTranscriptId };
  }

  return {
    transcripts: [
      ...state.transcripts,
      { id: state.nextTranscriptId, direction, text, final },
    ],
    nextTranscriptId: state.nextTranscriptId + 1,
  };
}

function sortSlots(slots: LayoutSlot[], removedSurfaceIds: readonly string[]): LayoutSlot[] {
  const removed = new Set(removedSurfaceIds);
  const seen = new Set<string>();
  const uniqueSlots: LayoutSlot[] = [];
  for (const slot of slots) {
    if (removed.has(slot.surfaceId) || seen.has(slot.surfaceId)) {
      continue;
    }
    seen.add(slot.surfaceId);
    uniqueSlots.push(slot);
  }
  return uniqueSlots.sort((left, right) => left.order - right.order);
}

export function dashboardReducer(state: DashboardState, action: DashboardAction): DashboardState {
  if (action.type === "local_status") {
    return {
      ...state,
      status: { message: action.message, tone: action.tone ?? "working" },
    };
  }

  const frame = action.frame;
  switch (frame.type) {
    case "ready":
      return {
        ...state,
        canvasGeneration: state.canvasGeneration + 1,
        pendingCards: [],
        removedSurfaceIds: [],
        sessionId: frame.sessionId,
        slots: [],
        status: { message: `Ready: ${frame.sessionId}`, tone: "ready" },
      };
    case "input_transcript": {
      const transcript = replaceTranscript(state, "input", frame.text, frame.final);
      return { ...state, ...transcript };
    }
    case "output_transcript": {
      const transcript = replaceTranscript(state, "output", frame.text, frame.final);
      return { ...state, ...transcript };
    }
    case "interrupted":
      return {
        ...state,
        status: { message: "Playback interrupted by new speech", tone: "working" },
      };
    case "usage":
      // Usage telemetry is intentionally not exposed as a user-facing metric.
      return state;
    case "layout":
      return {
        ...state,
        slots: sortSlots(frame.slots, state.removedSurfaceIds),
      };
    case "task_status":
      return {
        ...state,
        pendingCards: frame.status === "failed"
          ? state.pendingCards.filter((pending) => pending.taskId !== frame.taskId)
          : state.pendingCards,
        tasks: {
          ...state.tasks,
          [frame.taskId]: {
            taskId: frame.taskId,
            status: frame.status,
            ...(frame.surfaceId ? { surfaceId: frame.surfaceId } : {}),
            ...(frame.detail ? { detail: frame.detail } : {}),
          },
        },
      };
    case "card_pending": {
      const existingIndex = state.pendingCards.findIndex(
        (pending) => pending.surfaceId === frame.surfaceId,
      );
      const pending = {
        surfaceId: frame.surfaceId,
        taskId: frame.taskId,
        ...(frame.title ? { title: frame.title } : {}),
      };
      const pendingCards = existingIndex === -1
        ? [...state.pendingCards, pending]
        : state.pendingCards.map((item, index) => index === existingIndex ? pending : item);
      return { ...state, pendingCards };
    }
    case "card_removed": {
      const removedSurfaceIds = state.removedSurfaceIds.includes(frame.surfaceId)
        ? state.removedSurfaceIds
        : [...state.removedSurfaceIds, frame.surfaceId];
      return {
        ...state,
        pendingCards: state.pendingCards.filter(
          (pending) => pending.surfaceId !== frame.surfaceId,
        ),
        removedSurfaceIds,
        slots: state.slots.filter((slot) => slot.surfaceId !== frame.surfaceId),
      };
    }
    case "error":
      return {
        ...state,
        status: {
          message: frame.message,
          tone: "error",
        },
      };
    case "a2ui": {
      const wasPending = state.pendingCards.some(
        (pending) => pending.surfaceId === frame.surfaceId,
      );
      if (!wasPending) return state;
      const alreadySlotted = state.slots.some((slot) => slot.surfaceId === frame.surfaceId);
      const nextOrder = state.slots.reduce(
        (maximum, slot) => Math.max(maximum, slot.order),
        -1,
      ) + 1;
      return {
        ...state,
        pendingCards: state.pendingCards.filter(
          (pending) => pending.surfaceId !== frame.surfaceId,
        ),
        slots: alreadySlotted
          ? state.slots
          : [...state.slots, { surfaceId: frame.surfaceId, order: nextOrder }],
      };
    }
  }
}
