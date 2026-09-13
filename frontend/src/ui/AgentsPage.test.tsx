// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import App from "../App";

const roster = {
  agents: [
    {
      id: "live-host",
      name: "Live Host",
      kind: "live",
      model: "gemini-3.6-flash",
      description: "Runs the live voice session.",
      tools: [{ name: "delegate", description: "Send work to a specialist" }],
      dataSources: [
        { name: "Drive", detail: "Team documents", configured: true },
        { name: "Calendar", detail: "Upcoming events", configured: false },
      ],
      voiceBack: true,
    },
    {
      id: "researcher",
      name: "Researcher",
      kind: "llm",
      description: "Finds and synthesizes information.",
      tools: [{ name: "search", description: "Search the web" }],
      dataSources: [],
    },
  ],
};

beforeEach(() => {
  window.history.replaceState({}, "", "/?mock=1");
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  window.history.replaceState({}, "", "/");
});

describe("Agents page", () => {
  it("opens from the corner action, renders the fetched roster, and restores focus on Escape", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(roster),
    }));
    render(<App />);

    const opener = screen.getByRole("button", { name: "Agents" });
    opener.focus();
    fireEvent.click(opener);

    expect(screen.getByRole("dialog", { name: "Agents" })).toBeTruthy();
    expect(screen.getByLabelText("Loading agents")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Close Agents" })).toBe(document.activeElement);

    expect(await screen.findByRole("heading", { name: "Live Host" })).toBeTruthy();
    expect(screen.getByRole("heading", { name: "Researcher" })).toBeTruthy();
    expect(screen.getByText("gemini-3.6-flash")).toBeTruthy();
    expect(screen.getByText("worker")).toBeTruthy();
    expect(screen.getByText("delegate").getAttribute("title"))
      .toBe("Send work to a specialist");

    const configured = screen.getByLabelText("Drive: Configured");
    const unconfigured = screen.getByLabelText("Calendar: Not configured");
    expect(configured.getAttribute("data-configured")).toBe("true");
    expect(unconfigured.getAttribute("data-configured")).toBe("false");
    expect(screen.getByLabelText("Results are spoken through the live session")).toBeTruthy();

    fireEvent.keyDown(window, { key: "Escape" });
    await waitFor(() => expect(screen.queryByRole("dialog", { name: "Agents" })).toBeNull());
    expect(opener).toBe(document.activeElement);
  });

  it("shows a friendly retry state when the roster cannot be fetched", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("backend unavailable")));
    render(<App />);

    fireEvent.click(screen.getByRole("button", { name: "Agents" }));

    expect((await screen.findByRole("alert")).textContent)
      .toContain("Couldn’t reach the agent roster");
    expect(screen.getByRole("button", { name: "Try again" })).toBeTruthy();
  });
});
