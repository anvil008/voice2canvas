// @vitest-environment jsdom
import { act, cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { mockA2uiMessages } from "../mock";
import type { ServerFrame } from "../protocol";
import { dashboardReducer, initialDashboardState } from "../reducer";
import { A2uiCanvas, A2uiRuntime } from "./renderer";

afterEach(cleanup);

describe("A2uiCanvas", () => {
  it("renders the mock card and reacts to its updateDataModel patch", async () => {
    const runtime = new A2uiRuntime(() => undefined);
    const creation = mockA2uiMessages.filter(
      (message) =>
        (("createSurface" in message && message.createSurface.surfaceId === "card_brief") ||
          ("updateComponents" in message && message.updateComponents.surfaceId === "card_brief") ||
          ("updateDataModel" in message &&
            message.updateDataModel.surfaceId === "card_brief" &&
            message.updateDataModel.path === "/")),
    );
    const patch = mockA2uiMessages.filter(
      (message) =>
        "updateDataModel" in message &&
        message.updateDataModel.surfaceId === "card_brief" &&
        message.updateDataModel.path === "/summary",
    );

    act(() => runtime.feed(creation));
    const view = render(
      <A2uiCanvas runtime={runtime} slots={[{ surfaceId: "card_brief", order: 0 }]} />,
    );
    expect(screen.getByText("Morning briefing")).toBeTruthy();
    expect(screen.getByText("Gathering workspace context…")).toBeTruthy();

    await act(async () => {
      runtime.feed(patch);
      await Promise.resolve();
      await Promise.resolve();
    });
    expect(await screen.findByText("Brief complete — ready for review.")).toBeTruthy();

    view.rerender(
      <A2uiCanvas
        runtime={runtime}
        slots={[{ surfaceId: "card_brief", order: 0, span: 2 }]}
      />,
    );
    const slot = view.container.querySelector('[data-surface-id="card_brief"]');
    expect(slot?.className).toContain("surface-slot--span-2");
    expect(slot?.getAttribute("data-span")).toBe("2");
    runtime.dispose();
  });

  it("renders exactly the server replay after a second ready frame", () => {
    const runtime = new A2uiRuntime(() => undefined);
    let state = initialDashboardState;
    const bootstrapFor = (surfaceId: string) => mockA2uiMessages.filter(
      (message) =>
        !("deleteSurface" in message) &&
        (("createSurface" in message && message.createSurface.surfaceId === surfaceId) ||
          ("updateComponents" in message && message.updateComponents.surfaceId === surfaceId) ||
          ("updateDataModel" in message && message.updateDataModel.surfaceId === surfaceId)),
    );
    const apply = (frame: ServerFrame) => {
      if (frame.type === "ready") runtime.reset();
      if (frame.type === "a2ui") runtime.feed(frame.messages);
      state = dashboardReducer(state, { type: "server", frame });
    };

    apply({ type: "ready", sessionId: "first" });
    apply({ type: "a2ui", surfaceId: "card_brief", messages: bootstrapFor("card_brief") });
    apply({ type: "a2ui", surfaceId: "card_tasks", messages: bootstrapFor("card_tasks") });
    apply({
      type: "layout",
      slots: [
        { surfaceId: "card_brief", order: 0 },
        { surfaceId: "card_tasks", order: 1 },
      ],
    });

    const view = render(
      <A2uiCanvas
        key={state.canvasGeneration}
        pendingCards={state.pendingCards}
        runtime={runtime}
        slots={state.slots}
      />,
    );
    expect(view.container.querySelectorAll("[data-surface-id]")).toHaveLength(2);

    act(() => {
      apply({ type: "ready", sessionId: "second" });
      apply({ type: "a2ui", surfaceId: "card_tasks", messages: bootstrapFor("card_tasks") });
      apply({ type: "layout", slots: [{ surfaceId: "card_tasks", order: 0 }] });
      view.rerender(
        <A2uiCanvas
          key={state.canvasGeneration}
          pendingCards={state.pendingCards}
          runtime={runtime}
          slots={state.slots}
        />,
      );
    });

    expect(view.container.querySelectorAll("[data-surface-id]")).toHaveLength(1);
    expect(view.container.querySelector("[data-surface-id]")?.getAttribute("data-surface-id"))
      .toBe("card_tasks");
    expect(screen.queryByText("Morning briefing")).toBeNull();
    expect(screen.getAllByText("Suggested next steps")).toHaveLength(1);
    runtime.dispose();
  });
});
