// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { createElement } from "react";
import { describe, expect, it } from "vitest";
import { Gauge, toneForGauge } from "./Gauge";

describe("Gauge thresholds", () => {
  const thresholds = [
    { upTo: 2, tone: "positive" as const },
    { upTo: 5, tone: "warning" as const },
    { upTo: 11, tone: "negative" as const },
  ];

  it("selects the first ordered threshold containing the value", () => {
    expect(toneForGauge(2, thresholds)).toBe("positive");
    expect(toneForGauge(4, thresholds)).toBe("warning");
    expect(toneForGauge(8, thresholds)).toBe("negative");
    expect(toneForGauge(12, thresholds)).toBe("negative");
  });

  it("is neutral when no thresholds are supplied", () => {
    expect(toneForGauge(4)).toBe("neutral");
  });

  it("renders safely when value or max is null/undefined", () => {
    render(
      createElement(Gauge, {
        label: "Test Gauge",
        value: undefined as unknown as number,
        max: undefined as unknown as number,
      }),
    );
    expect(screen.getByText("Test Gauge")).toBeTruthy();
    expect(screen.getAllByText(/—/).length).toBeGreaterThan(0);
  });
});
