// @vitest-environment jsdom
import { act, cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import {
  extendedComparisonCreationMessages,
  extendedGaugeCreationMessages,
  extendedWeatherCreationMessages,
  extendedWeatherUpdateMessages,
} from "../../mock";
import { A2uiCanvas, A2uiRuntime } from "../renderer";

afterEach(cleanup);

describe("extended catalog rendering", () => {
  it("resolves bindings and re-renders a Stat and LineChart after a model patch", async () => {
    const runtime = new A2uiRuntime(() => undefined);
    act(() => runtime.feed(extendedWeatherCreationMessages));
    const { container } = render(
      <A2uiCanvas runtime={runtime} slots={[{ surfaceId: "card_weather", order: 0 }]} />,
    );

    expect(container.querySelector(".extended-stat")?.textContent).toContain("22");
    const originalPath = container.querySelector("path[data-series]")?.getAttribute("d");
    expect(originalPath).toBeTruthy();

    await act(async () => {
      runtime.feed(extendedWeatherUpdateMessages);
      await Promise.resolve();
    });
    expect(container.querySelector(".extended-stat")?.textContent).toContain("25");
    expect(container.querySelector("path[data-series]")?.getAttribute("d")).not.toBe(originalPath);
    runtime.dispose();
  });

  it("renders the complete extended mock act", () => {
    const runtime = new A2uiRuntime(() => undefined);
    act(() => {
      runtime.feed(extendedWeatherCreationMessages);
      runtime.feed(extendedComparisonCreationMessages);
      runtime.feed(extendedGaugeCreationMessages);
    });
    const { container } = render(
      <A2uiCanvas
        runtime={runtime}
        slots={[
          { surfaceId: "card_weather", order: 0 },
          { surfaceId: "card_comparison", order: 1 },
          { surfaceId: "card_uv", order: 2 },
        ]}
      />,
    );

    expect(screen.getByText("Calgary forecast")).toBeTruthy();
    expect(screen.getByText("Weekend comparison")).toBeTruthy();
    expect(screen.getByText("UV outlook")).toBeTruthy();
    expect(screen.getByText("Protection required")).toBeTruthy();
    expect(container.querySelectorAll("path[data-series]").length).toBeGreaterThan(0);
    expect(container.querySelectorAll("[data-bar-fill]").length).toBeGreaterThan(0);
    expect(container.querySelector(".extended-gauge[data-tone='negative']")).toBeTruthy();
    runtime.dispose();
  });

  it("renders long StatGroup labels without a fixed-width truncation hook", () => {
    const runtime = new A2uiRuntime(() => undefined);
    act(() => runtime.feed(extendedWeatherCreationMessages));
    const { container } = render(
      <A2uiCanvas runtime={runtime} slots={[{ surfaceId: "card_weather", order: 0 }]} />,
    );

    const label = screen.getByText("Modernization candidates requiring architectural review");
    expect(label.className).toBe("extended-stat__label");
    expect(label.className).not.toMatch(/truncate|ellipsis|nowrap/);
    expect(container.querySelector(".extended-stat-group")?.getAttribute("style"))
      .toContain("auto-fit");
    expect(screen.getByText(/advisory-reference-identifier/).closest("dd")?.className)
      .not.toMatch(/truncate|ellipsis|nowrap/);
    runtime.dispose();
  });

  it("renders warning chips for unknown and malformed extended components", () => {
    const runtime = new A2uiRuntime(() => undefined);
    act(() => runtime.feed([
      {
        version: "v0.9.1",
        createSurface: {
          surfaceId: "card_warning",
          catalogId: "https://voice2canvas.local/catalogs/extended/v1",
        },
      },
      {
        version: "v0.9.1",
        updateComponents: {
          surfaceId: "card_warning",
          components: [
            { id: "root", component: "Card", child: "body" },
            { id: "body", component: "Column", children: ["unknown", "bad_stat"] },
            { id: "unknown", component: "PieChart", values: [1, 2] },
            { id: "bad_stat", component: "Stat", label: "Missing value" },
          ],
        },
      },
    ]));
    render(<A2uiCanvas runtime={runtime} slots={[{ surfaceId: "card_warning", order: 0 }]} />);
    expect(screen.getByText("Unsupported extended component: PieChart")).toBeTruthy();
    expect(screen.getByText(/Invalid Stat/)).toBeTruthy();
    runtime.dispose();
  });

  it("accepts legacy catalog ID alias", () => {
    const runtime = new A2uiRuntime(() => undefined);
    act(() => runtime.feed([
      {
        version: "v0.9.1",
        createSurface: {
          surfaceId: "card_legacy",
          catalogId: "https://v2ui.local/catalogs/extended/v1",
        },
      },
      {
        version: "v0.9.1",
        updateComponents: {
          surfaceId: "card_legacy",
          components: [
            { id: "root", component: "Card", child: "stat" },
            { id: "stat", component: "Stat", label: "Speed", value: 42 },
          ],
        },
      },
    ]));
    render(<A2uiCanvas runtime={runtime} slots={[{ surfaceId: "card_legacy", order: 0 }]} />);
    expect(screen.getByText("Speed")).toBeTruthy();
    expect(screen.getByText("42")).toBeTruthy();
    runtime.dispose();
  });
});
