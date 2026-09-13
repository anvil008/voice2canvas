// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { MicCapture } from "./mic";

describe("MicCapture", () => {
  const originalMediaDevices = navigator.mediaDevices;
  const originalIsSecureContext = window.isSecureContext;

  beforeEach(() => {
    Object.defineProperty(navigator, "mediaDevices", {
      value: undefined,
      configurable: true,
      writable: true,
    });
  });

  afterEach(() => {
    Object.defineProperty(navigator, "mediaDevices", {
      value: originalMediaDevices,
      configurable: true,
      writable: true,
    });
    Object.defineProperty(window, "isSecureContext", {
      value: originalIsSecureContext,
      configurable: true,
      writable: true,
    });
  });

  it("throws actionable HTTPS error when isSecureContext is false and mediaDevices is unavailable", async () => {
    Object.defineProperty(window, "isSecureContext", {
      value: false,
      configurable: true,
      writable: true,
    });

    const mic = new MicCapture();
    await expect(mic.start(() => {})).rejects.toThrow(
      "Microphone requires a secure HTTPS connection. Please open this page using https://.",
    );
  });

  it("throws general unsupported error when isSecureContext is true but mediaDevices is unavailable", async () => {
    Object.defineProperty(window, "isSecureContext", {
      value: true,
      configurable: true,
      writable: true,
    });

    const mic = new MicCapture();
    await expect(mic.start(() => {})).rejects.toThrow(
      "This browser does not support microphone capture.",
    );
  });
});
