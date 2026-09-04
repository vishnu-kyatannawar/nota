// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import { UpdateConsentDialog } from "./UpdateConsentDialog";

// jsdom has no native <dialog> behaviour.
beforeAll(() => {
  HTMLDialogElement.prototype.showModal = function () { this.open = true; };
  HTMLDialogElement.prototype.close = function () { this.open = false; };
});
afterEach(cleanup);

describe("update consent", () => {
  it("says what will happen before anything is sent", () => {
    render(<UpdateConsentDialog open onAnswer={vi.fn()} />);
    const text = document.body.textContent ?? "";
    expect(text).toContain("GitHub");
    expect(text).toContain("only time Nota uses the network");
  });

  it("records yes as automatic checking", () => {
    const onAnswer = vi.fn();
    render(<UpdateConsentDialog open onAnswer={onAnswer} />);
    fireEvent.click(screen.getByText("Yes, check"));
    expect(onAnswer).toHaveBeenCalledWith("auto");
  });

  it("records no, and treats dismissing as no", () => {
    const onAnswer = vi.fn();
    const { rerender } = render(<UpdateConsentDialog open onAnswer={onAnswer} />);
    fireEvent.click(screen.getByText("No, stay offline"));
    expect(onAnswer).toHaveBeenCalledWith("never");

    // Closing without choosing must not be read as consent.
    onAnswer.mockClear();
    rerender(<UpdateConsentDialog open onAnswer={onAnswer} />);
    fireEvent.keyDown(document.querySelector("dialog") as HTMLDialogElement, { key: "Escape" });
    fireEvent.click(document.querySelector("dialog") as HTMLDialogElement);
    expect(onAnswer.mock.calls.every(([c]) => c === "never")).toBe(true);
  });
});
