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
});
