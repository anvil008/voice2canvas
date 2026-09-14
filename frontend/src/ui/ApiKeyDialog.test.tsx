// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiKeyDialog } from "./ApiKeyDialog";

afterEach(cleanup);

describe("ApiKeyDialog", () => {
  it("requires a key and returns the trimmed value", () => {
    const onSave = vi.fn();
    render(<ApiKeyDialog onCancel={() => undefined} onSave={onSave} />);
    expect(screen.getByText(/Voice2Canvas keeps it in this tab's/)).toBeTruthy();

    const save = screen.getByRole("button", { name: "Use this key" });
    expect(save.hasAttribute("disabled")).toBe(true);
    fireEvent.change(screen.getByLabelText("Gemini API key"), { target: { value: "  test-key  " } });
    fireEvent.click(save);
    expect(onSave).toHaveBeenCalledWith("test-key");
  });

  it("closes when Escape key is pressed", () => {
    const onCancel = vi.fn();
    render(<ApiKeyDialog onCancel={onCancel} onSave={() => undefined} />);
    fireEvent.keyDown(window, { key: "Escape" });
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it("renders Try Demo Mode button", () => {
    render(<ApiKeyDialog onCancel={() => undefined} onSave={() => undefined} />);
    const demoButton = screen.getByRole("button", { name: "Try Demo Mode" });
    expect(demoButton).toBeTruthy();
  });
});
