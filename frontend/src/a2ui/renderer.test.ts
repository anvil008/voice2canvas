import { describe, expect, it } from "vitest";
import { mockA2uiMessages } from "../mock";
import { A2uiRuntime, PROTOCOL_BASIC_CATALOG_ID } from "./renderer";

describe("A2uiRuntime", () => {
  it("accepts v0.9.1 fixture messages, forwards actions, patches data, and removes a surface", async () => {
    const actions: string[] = [];
    const runtime = new A2uiRuntime((action) => actions.push(action.name));
    const creation = mockA2uiMessages.filter(
      (message) =>
        ("createSurface" in message || "updateComponents" in message || "updateDataModel" in message) &&
        (("createSurface" in message && message.createSurface.surfaceId === "card_brief") ||
          ("updateComponents" in message && message.updateComponents.surfaceId === "card_brief") ||
          ("updateDataModel" in message &&
            message.updateDataModel.surfaceId === "card_brief" &&
            message.updateDataModel.path === "/")),
    );

    runtime.feed(creation);
    expect(PROTOCOL_BASIC_CATALOG_ID).toContain("v0_9_1");
    expect(runtime.getSurface("card_brief")?.dataModel.get("/summary")).toBe(
      "Gathering workspace context…",
    );

    await runtime
      .getSurface("card_brief")
      ?.dispatchAction({ event: { name: "open_task", context: { taskId: "task_brief" } } }, "run");
    expect(actions).toEqual(["open_task"]);

    const patch = mockA2uiMessages.filter(
      (message) =>
        "updateDataModel" in message &&
        message.updateDataModel.surfaceId === "card_brief" &&
        message.updateDataModel.path === "/summary",
    );
    runtime.feed(patch);
    expect(runtime.getSurface("card_brief")?.dataModel.get("/summary")).toBe(
      "Brief complete — ready for review.",
    );

    const deletion = mockA2uiMessages.filter(
      (message) => "deleteSurface" in message && message.deleteSurface.surfaceId === "card_brief",
    );
    runtime.feed(deletion);
    expect(runtime.getSurface("card_brief")).toBeUndefined();
    runtime.dispose();
  });

  it("re-bootstraps an existing surface without stale data-model state", () => {
    const runtime = new A2uiRuntime(() => undefined);
    const creation = mockA2uiMessages.filter(
      (message) =>
        (("createSurface" in message || "updateComponents" in message || "updateDataModel" in message) &&
          !(
            "updateDataModel" in message &&
            message.updateDataModel.path !== "/"
          )) &&
        (("createSurface" in message && message.createSurface.surfaceId === "card_brief") ||
          ("updateComponents" in message && message.updateComponents.surfaceId === "card_brief") ||
          ("updateDataModel" in message && message.updateDataModel.surfaceId === "card_brief")),
    );

    runtime.feed(creation);
    runtime.feed([
      {
        version: "v0.9.1",
        updateDataModel: { surfaceId: "card_brief", path: "/stale", value: true },
      },
    ]);
    expect(runtime.getSurface("card_brief")?.dataModel.get("/stale")).toBe(true);

    const bootstrapWithoutData = creation.filter((message) => !("updateDataModel" in message));
    expect(() => runtime.feed(bootstrapWithoutData)).not.toThrow();
    expect(runtime.getSurface("card_brief")?.dataModel.get("/stale")).toBeUndefined();

    runtime.reset();
    expect(runtime.getSurface("card_brief")).toBeUndefined();
    runtime.dispose();
  });
});
