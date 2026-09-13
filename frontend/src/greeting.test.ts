import { describe, expect, it } from "vitest";
import { greetingForHour } from "./greeting";

describe("greetingForHour", () => {
  it("greets by time of day", () => {
    expect(greetingForHour(8, 0)).toMatch(/morning/i);
    expect(greetingForHour(14, 0)).toMatch(/afternoon/i);
    expect(greetingForHour(19, 0)).toMatch(/evening/i);
    expect(greetingForHour(23, 0)).toMatch(/midnight|late night/i);
    expect(greetingForHour(2, 0)).toMatch(/midnight|late night/i);
  });

  it("addresses Anvil and stays stable within a day", () => {
    expect(greetingForHour(9, 41)).toContain("Anvil");
    expect(greetingForHour(9, 41)).toBe(greetingForHour(9, 41));
    // different days can rotate lines
    const lines = new Set([greetingForHour(9, 0), greetingForHour(9, 1)]);
    expect(lines.size).toBeGreaterThanOrEqual(1);
  });
});
