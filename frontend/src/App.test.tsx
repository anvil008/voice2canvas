// @vitest-environment jsdom
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import App from "./App";
import { MOCK_PLAYBACK_RATE } from "./mock";

function advanceMockTime(milliseconds: number): void {
  vi.advanceTimersByTime(milliseconds * MOCK_PLAYBACK_RATE);
}

beforeEach(() => {
  window.localStorage.clear();
  document.documentElement.removeAttribute("data-theme");
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
  window.history.replaceState({}, "", "/");
});

describe("mock mode", () => {

  it("renders one cohesive dock with every cluster and an expandable conversation drawer", async () => {
    vi.useFakeTimers();
    window.history.replaceState({}, "", "/?mock=1");
    render(<App />);

    expect(screen.getByRole("heading", { level: 1 })).toBeTruthy();
    expect(screen.getByLabelText("Voice2Canvas")).toBeTruthy();
    expect(screen.getByText("Voice2Canvas")).toBeTruthy();
    const transcriptToggle = screen.getByRole("button", { name: /conversation/i });
    expect(transcriptToggle.getAttribute("aria-expanded")).toBe("false");
    expect(document.querySelector(".corner-actions .metric-chip")).toBeNull();
    expect(document.querySelector(".voice-context-row")).toBeNull();
    expect(document.querySelector(".transcript-peek")).toBeNull();
    expect(document.querySelectorAll(".voice-layer > .voice-dock-shell")).toHaveLength(1);
    expect(screen.getByTestId("dock-mic-cluster")).toBeTruthy();
    expect(screen.getByTestId("dock-status-cluster")).toBeTruthy();
    expect(screen.getByTestId("dock-controls-cluster")).toBeTruthy();
    expect(document.querySelector(".voice-metrics")).toBeNull();
    expect(screen.queryByText("wpm")).toBeNull();
    expect(screen.queryByText("tok/m")).toBeNull();

    const themeToggle = screen.getByRole("button", { name: "Switch to dark theme" });
    fireEvent.click(themeToggle);
    expect(document.documentElement.dataset.theme).toBe("dark");
    expect(screen.getByRole("button", { name: "Switch to light theme" })).toBeTruthy();

    await act(async () => {
      advanceMockTime(1_750);
      await Promise.resolve();
      await Promise.resolve();
    });
    expect(screen.queryByRole("heading", { level: 1 })).toBeNull();
    expect(screen.getAllByText("I created your morning briefing and suggested next steps.").length).toBeGreaterThan(0);
    expect(screen.getByTestId("dock-status-cluster").textContent)
      .toContain("Assistant · I created your morning briefing and suggested next steps.");

    fireEvent.click(transcriptToggle);
    expect(transcriptToggle.getAttribute("aria-expanded")).toBe("true");
    expect(document.querySelector(".transcript-panel")?.getAttribute("data-open")).toBe("true");

    fireEvent.click(screen.getByRole("button", { name: "Collapse conversation" }));
    expect(document.querySelector(".transcript-panel")?.getAttribute("data-open")).toBe("false");

    fireEvent.click(screen.getByRole("button", { name: "Expand conversation" }));
    expect(document.querySelector(".transcript-panel")?.getAttribute("data-open")).toBe("true");
    fireEvent.keyDown(window, { key: "Escape" });
    expect(document.querySelector(".transcript-panel")?.getAttribute("data-open")).toBe("false");
    expect(screen.getByRole("button", { name: "Expand conversation" })).toBe(document.activeElement);
  });

  it("runs the canned protocol sequence through the rendered A2UI card path", async () => {
    vi.useFakeTimers();
    window.history.replaceState({}, "", "/?mock=1");
    render(<App />);

    await act(async () => {
      advanceMockTime(2_800);
      await Promise.resolve();
      await Promise.resolve();
    });
    expect(screen.getByText("Morning briefing")).toBeTruthy();
    expect(screen.getByText("Suggested next steps")).toBeTruthy();
    expect(screen.getByText("Brief complete — ready for review.")).toBeTruthy();

    await act(async () => {
      advanceMockTime(700);
      await Promise.resolve();
    });
    expect(screen.queryByText("Morning briefing")).toBeNull();
    expect(screen.getByText("Suggested next steps")).toBeTruthy();
  });

  it("replaces pending mock cards without injecting an artificial failure", async () => {
    vi.useFakeTimers();
    window.history.replaceState({}, "", "/?mock=1");
    const { container } = render(<App />);

    await act(async () => {
      advanceMockTime(800);
      await Promise.resolve();
    });
    expect(container.querySelector('[data-task-id="task_brief"]')).toBeTruthy();
    expect(container.querySelector(".task-toast.researching strong")?.textContent)
      .toBe("researching…");
    expect(container.querySelector('[data-task-id="task_brief"] .pending-card p')?.textContent)
      .toBe("researching…");

    await act(async () => {
      advanceMockTime(150);
      await Promise.resolve();
    });
    expect(screen.getByText("Generating")).toBeTruthy();
    expect(screen.getByText("generating…")).toBeTruthy();

    await act(async () => {
      advanceMockTime(150);
      await Promise.resolve();
      await Promise.resolve();
    });
    expect(container.querySelector('[data-task-id="task_brief"]')).toBeNull();
    expect(screen.getByText("Morning briefing")).toBeTruthy();

    await act(async () => {
      advanceMockTime(2_000);
      await Promise.resolve();
    });
    expect(container.querySelector('[data-task-id="task_failed"]')).toBeNull();
    expect(screen.queryByText("Flight options")).toBeNull();
    expect(screen.queryByText("No matching routes were returned")).toBeNull();
    expect(screen.queryByText("Failed")).toBeNull();
  });

  it("runs the extended mock act, applies its live patch, and remains rendered across themes", async () => {
    vi.useFakeTimers();
    window.history.replaceState({}, "", "/?mock=1");
    const { container } = render(<App />);

    await act(async () => {
      advanceMockTime(5_300);
      await Promise.resolve();
      await Promise.resolve();
    });

    expect(screen.getByText("Calgary forecast")).toBeTruthy();
    expect(screen.getByText("Weekend comparison")).toBeTruthy();
    expect(screen.getByText("UV outlook")).toBeTruthy();
    expect(container.querySelector(".extended-stat")?.textContent).toContain("25");
    expect(screen.queryByText("wpm")).toBeNull();
    expect(screen.queryByText("tok/m")).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "Switch to dark theme" }));
    expect(document.documentElement.dataset.theme).toBe("dark");
    expect(screen.getByText("Calgary forecast")).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "Switch to light theme" }));
    expect(document.documentElement.dataset.theme).toBe("light");
    expect(container.querySelector("path[data-series]")).toBeTruthy();
    const chartSlot = container.querySelector('[data-surface-id="card_weather"]');
    expect(chartSlot?.className).toContain("surface-slot--span-2");
    expect(chartSlot?.getAttribute("data-span")).toBe("2");
    expect((chartSlot as HTMLElement | null)?.style.order).toBe("0");
  });
});

describe("connect icon button", () => {
  it("renders connect button and transitions to is-connected when key is configured", () => {
    render(<App />);

    const connectButton = screen.getByRole("button", { name: "Connect Gemini" });
    expect(connectButton).toBeTruthy();
    expect(connectButton.getAttribute("title")).toBe("Connect Gemini");
    expect(connectButton.classList.contains("is-connected")).toBe(false);
    expect(connectButton.querySelector("svg")).toBeTruthy();

    fireEvent.click(connectButton);
    expect(screen.getByRole("dialog")).toBeTruthy();
    const input = screen.getByLabelText("Gemini API key");
    fireEvent.change(input, { target: { value: "test-api-key" } });
    fireEvent.click(screen.getByRole("button", { name: "Use this key" }));

    const connectedButton = screen.getByRole("button", { name: "Gemini connected" });
    expect(connectedButton).toBeTruthy();
    expect(connectedButton.getAttribute("title")).toBe("Gemini connected (click to replace key)");
    expect(connectedButton.classList.contains("is-connected")).toBe(true);
  });
});
