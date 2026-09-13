import { describe, expect, it } from "vitest";
import { dashboardReducer, initialDashboardState } from "./reducer";

describe("dashboardReducer", () => {
  it("replaces interim transcripts, preserves final text, and orders layout slots", () => {
    let state = dashboardReducer(initialDashboardState, {
      type: "server",
      frame: { type: "input_transcript", text: "build a", final: false },
    });
    state = dashboardReducer(state, {
      type: "server",
      frame: { type: "input_transcript", text: "build a card", final: false },
    });
    state = dashboardReducer(state, {
      type: "server",
      frame: { type: "input_transcript", text: "build a card", final: true },
    });
    state = dashboardReducer(state, {
      type: "server",
      frame: {
        type: "layout",
        slots: [
          { surfaceId: "second", order: 2, span: 2 },
          { surfaceId: "first", order: 1 },
        ],
      },
    });

    expect(state.transcripts).toEqual([
      { id: 1, direction: "input", text: "build a card", final: true },
    ]);
    expect(state.slots.map((slot) => slot.surfaceId)).toEqual(["first", "second"]);
    expect(state.slots[1]?.span).toBe(2);
  });

  it("drops a slot when the app-level card_removed signal arrives", () => {
    const withLayout = dashboardReducer(initialDashboardState, {
      type: "server",
      frame: {
        type: "layout",
        slots: [
          { surfaceId: "keep", order: 0 },
          { surfaceId: "remove", order: 1 },
        ],
      },
    });
    const removed = dashboardReducer(withLayout, {
      type: "server",
      frame: { type: "card_removed", surfaceId: "remove" },
    });

    expect(removed.slots).toEqual([{ surfaceId: "keep", order: 0 }]);
    expect(removed.removedSurfaceIds).toEqual(["remove"]);
  });

  it("replaces a pending card with a provisional live slot on its first a2ui frame", () => {
    const withLayout = dashboardReducer(initialDashboardState, {
      type: "server",
      frame: { type: "layout", slots: [{ surfaceId: "existing", order: 4 }] },
    });
    const pending = dashboardReducer(withLayout, {
      type: "server",
      frame: {
        type: "card_pending",
        surfaceId: "new-card",
        taskId: "new-task",
        title: "New analysis",
      },
    });

    expect(pending.pendingCards).toEqual([
      { surfaceId: "new-card", taskId: "new-task", title: "New analysis" },
    ]);

    const replaced = dashboardReducer(pending, {
      type: "server",
      frame: { type: "a2ui", surfaceId: "new-card", messages: [] },
    });
    expect(replaced.pendingCards).toEqual([]);
    expect(replaced.slots).toEqual([
      { surfaceId: "existing", order: 4 },
      { surfaceId: "new-card", order: 5 },
    ]);
  });

  it("removes a pending card when its task fails", () => {
    const pending = dashboardReducer(initialDashboardState, {
      type: "server",
      frame: { type: "card_pending", surfaceId: "failed-card", taskId: "failed-task" },
    });
    const failed = dashboardReducer(pending, {
      type: "server",
      frame: {
        type: "task_status",
        taskId: "failed-task",
        surfaceId: "failed-card",
        status: "failed",
      },
    });

    expect(failed.pendingCards).toEqual([]);
    expect(failed.tasks["failed-task"]?.status).toBe("failed");
  });

  it("tracks researching between dispatched and generating", () => {
    const researching = dashboardReducer(initialDashboardState, {
      type: "server",
      frame: {
        type: "task_status",
        taskId: "research-task",
        surfaceId: "research-card",
        status: "researching",
      },
    });

    expect(researching.tasks["research-task"]).toEqual({
      taskId: "research-task",
      surfaceId: "research-card",
      status: "researching",
    });
  });

  it("clears replay-owned slots and pendings on ready while preserving session context", () => {
    let state = dashboardReducer(initialDashboardState, {
      type: "server",
      frame: { type: "input_transcript", text: "keep this", final: true },
    });
    state = dashboardReducer(state, {
      type: "server",
      frame: { type: "task_status", taskId: "keep-task", status: "rendered" },
    });
    state = dashboardReducer(state, {
      type: "server",
      frame: { type: "card_pending", surfaceId: "pending", taskId: "pending-task" },
    });
    state = dashboardReducer(state, {
      type: "server",
      frame: { type: "layout", slots: [{ surfaceId: "old-card", order: 0 }] },
    });
    state = dashboardReducer(state, {
      type: "server",
      frame: { type: "card_removed", surfaceId: "removed-card" },
    });

    const replay = dashboardReducer(state, {
      type: "server",
      frame: { type: "ready", sessionId: "reconnected" },
    });

    expect(replay.canvasGeneration).toBe(1);
    expect(replay.slots).toEqual([]);
    expect(replay.pendingCards).toEqual([]);
    expect(replay.removedSurfaceIds).toEqual([]);
    expect(replay.transcripts).toEqual(state.transcripts);
    expect(replay.tasks).toEqual(state.tasks);
  });

});
