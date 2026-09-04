// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { Banner } from "./Banner";

afterEach(cleanup);

describe("Banner", () => {
  it("announces an error assertively and news politely", () => {
    render(<Banner tone="danger">went wrong</Banner>);
    expect(screen.getByRole("alert").textContent).toContain("went wrong");
    cleanup();
    render(<Banner tone="accent">4.2.0 is out</Banner>);
    expect(screen.getByRole("status").textContent).toContain("4.2.0 is out");
  });

  it("runs the action when it is pressed", () => {
    const onClick = vi.fn();
    render(<Banner tone="accent" action={{ label: "Install", onClick }}>x</Banner>);
    fireEvent.click(screen.getByText("Install"));
    expect(onClick).toHaveBeenCalledOnce();
  });

  it("disables the action while it is busy", () => {
    render(<Banner tone="accent" action={{ label: "Install", onClick: vi.fn(), busy: true }}>x</Banner>);
    expect((screen.getByText("Install") as HTMLButtonElement).disabled).toBe(true);
  });

  it("dismisses only when it can be dismissed", () => {
    const onDismiss = vi.fn();
    render(<Banner tone="danger" onDismiss={onDismiss}>x</Banner>);
    fireEvent.click(screen.getByText("dismiss"));
    expect(onDismiss).toHaveBeenCalledOnce();
    cleanup();
    render(<Banner tone="danger">x</Banner>);
    expect(screen.queryByText("dismiss")).toBeNull();
  });
});
