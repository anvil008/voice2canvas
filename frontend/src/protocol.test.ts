import { describe, expect, it } from "vitest";
import { parseServerFrame } from "./protocol";

describe("parseServerFrame", () => {
  it("parses card_pending frames with and without a title", () => {
    expect(parseServerFrame({
      type: "card_pending",
      surfaceId: "card_1",
      taskId: "task_1",
      title: "Weather",
    })).toEqual({
      type: "card_pending",
      surfaceId: "card_1",
      taskId: "task_1",
      title: "Weather",
    });
    expect(parseServerFrame({
      type: "card_pending",
      surfaceId: "card_2",
      taskId: "task_2",
    })).toEqual({
      type: "card_pending",
      surfaceId: "card_2",
      taskId: "task_2",
    });
  });

  it("rejects card_pending frames missing required identifiers", () => {
    expect(parseServerFrame({ type: "card_pending", taskId: "task_1" })).toBeUndefined();
    expect(parseServerFrame({ type: "card_pending", surfaceId: "card_1" })).toBeUndefined();
  });

  it("parses layout spans and researching task statuses", () => {
    expect(parseServerFrame({
      type: "layout",
      slots: [
        { surfaceId: "chart", order: 1, span: 2 },
        { surfaceId: "summary", order: 0 },
      ],
    })).toEqual({
      type: "layout",
      slots: [
        { surfaceId: "chart", order: 1, span: 2 },
        { surfaceId: "summary", order: 0 },
      ],
    });
    expect(parseServerFrame({
      type: "task_status",
      taskId: "task_1",
      status: "researching",
    })).toEqual({ type: "task_status", taskId: "task_1", status: "researching" });
  });

  it("rejects unsupported layout spans", () => {
    expect(parseServerFrame({
      type: "layout",
      slots: [{ surfaceId: "chart", order: 0, span: 3 }],
    })).toBeUndefined();
  });

  it("parses usage frames and rejects invalid token counts or timestamps", () => {
    expect(parseServerFrame({
      type: "usage",
      inputTokens: 120,
      outputTokens: 45,
      source: "worker",
      at: "2026-08-01T18:00:00.000Z",
    })).toEqual({
      type: "usage",
      inputTokens: 120,
      outputTokens: 45,
      source: "worker",
      at: "2026-08-01T18:00:00.000Z",
    });
    expect(parseServerFrame({
      type: "usage",
      inputTokens: 120,
      outputTokens: -1,
      source: "worker",
      at: "2026-08-01T18:00:00.000Z",
    })).toBeUndefined();
    expect(parseServerFrame({
      type: "usage",
      inputTokens: 120,
      outputTokens: 45,
      source: "batch",
      at: "not-a-date",
    })).toBeUndefined();
  });
});
