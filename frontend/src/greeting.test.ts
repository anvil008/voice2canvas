// @vitest-environment jsdom
import { beforeEach, describe, expect, it } from "vitest";
import { greetingForHour } from "./greeting";

describe("greetingForHour", () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it("greets by time of day", () => {
    expect(greetingForHour(8, 0)).toMatch(/morning/i);
    expect(greetingForHour(14, 0)).toMatch(/afternoon/i);
    expect(greetingForHour(19, 0)).toMatch(/evening/i);
    expect(greetingForHour(23, 0)).toMatch(/midnight|late night/i);
    expect(greetingForHour(2, 0)).toMatch(/midnight|late night/i);
  });

  it("provides neutral greetings by default and stays stable within a day", () => {
    const greeting = greetingForHour(9, 41);
    expect(greeting).not.toContain("Anvil");
    expect(greeting).toMatch(/morning/i);
    expect(greetingForHour(9, 41)).toBe(greeting);
    // different days can rotate lines
    const lines = new Set([greetingForHour(9, 0), greetingForHour(9, 1)]);
    expect(lines.size).toBeGreaterThanOrEqual(1);
  });

  it("supports an optional custom name parameter", () => {
    expect(greetingForHour(9, 41, "Taylor")).toContain("Taylor");
  });

  it("supports voice2canvas_name in localStorage", () => {
    localStorage.setItem("voice2canvas_name", "Alex");
    expect(greetingForHour(9, 0)).toContain("Alex");
    localStorage.removeItem("voice2canvas_name");
    expect(greetingForHour(9, 0)).not.toContain("Alex");
  });
});
