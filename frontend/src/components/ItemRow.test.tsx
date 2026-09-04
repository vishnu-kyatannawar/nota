// @vitest-environment jsdom
import { act, cleanup, render } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ItemRow } from "./ItemRow";
import { blankItem } from "../lib/items";
import { ITEM_ROW_ATTR } from "../lib/focus";

afterEach(() => {
  cleanup();
  document.body.innerHTML = "";
});

function row(text = "task") {
  return render(
    <ItemRow item={{ ...blankItem(0, text) }} index={0} focused caret="end" dispatch={vi.fn()} dark={false} allLabels={[]} />,
  );
}

describe("item row focus", () => {
  it("takes focus when nothing else is being typed in", () => {
    const { getByLabelText } = row();
    expect(document.activeElement).toBe(getByLabelText("Item text"));
  });

  it("leaves the caret alone when the user is typing in the notes editor", () => {
    const notes = document.createElement("div");
    notes.setAttribute("contenteditable", "true");
    notes.tabIndex = 0;
    document.body.appendChild(notes);
    notes.focus();

    row();
    expect(document.activeElement).toBe(notes);
  });

  it("leaves the caret alone when the user is renaming in the sidebar", () => {
    const rename = document.createElement("input");
    document.body.appendChild(rename);
    rename.focus();

    row();
    expect(document.activeElement).toBe(rename);
  });

  it("still moves between item rows, which is what Enter and the arrows do", () => {
    const sibling = document.createElement("input");
    sibling.setAttribute(ITEM_ROW_ATTR, "");
    document.body.appendChild(sibling);
    sibling.focus();

    const { getByLabelText } = row();
    expect(document.activeElement).toBe(getByLabelText("Item text"));
  });

  it("applies the same rule to a heading row", () => {
    const notes = document.createElement("div");
    notes.setAttribute("contenteditable", "true");
    notes.tabIndex = 0;
    document.body.appendChild(notes);
    notes.focus();

    render(
      <ItemRow item={blankItem(0, "Must", "heading")} index={0} focused caret="end" dispatch={vi.fn()} dark={false} allLabels={[]} />,
    );
    expect(document.activeElement).toBe(notes);
  });
});

describe("a long item", () => {
  // jsdom does no layout, so scrollHeight is always 0. Stubbing it is what
  // makes the sizing observable at all.
  function withScrollHeight(px: number) {
    Object.defineProperty(HTMLTextAreaElement.prototype, "scrollHeight", {
      configurable: true,
      get: () => px,
    });
  }

  afterEach(() => {
    Reflect.deleteProperty(HTMLTextAreaElement.prototype, "scrollHeight");
  });

  it("grows to fit its text instead of hiding it on one line", () => {
    withScrollHeight(66);
    const { getByLabelText } = row("a sentence long enough to wrap several times over");
    expect((getByLabelText("Item text") as HTMLTextAreaElement).style.height).toBe("66px");
  });

  it("measures again once the bundled fonts have loaded", async () => {
    let ready: () => void = () => {};
    const fonts = { ready: new Promise<void>((r) => { ready = () => r(); }) };
    Object.defineProperty(document, "fonts", { configurable: true, value: fonts });
    withScrollHeight(22);

    const { getByLabelText } = row("wraps once the real face arrives");
    const field = getByLabelText("Item text") as HTMLTextAreaElement;
    expect(field.style.height).toBe("22px");

    // The face arrives and the text now needs two lines.
    withScrollHeight(44);
    await act(async () => {
      ready();
      await fonts.ready;
    });
    expect(field.style.height).toBe("44px");
    Reflect.deleteProperty(document, "fonts");
  });

  it("measures again when the column it sits in changes width", () => {
    const observers: (() => void)[] = [];
    class FakeObserver {
      constructor(private readonly cb: () => void) { observers.push(() => this.cb()); }
      observe() {}
      disconnect() {}
    }
    vi.stubGlobal("ResizeObserver", FakeObserver);
    withScrollHeight(22);

    const { getByLabelText } = row("rewraps when the pane narrows");
    const field = getByLabelText("Item text") as HTMLTextAreaElement;
    expect(field.style.height).toBe("22px");

    // Side by side, or a narrower window: the text wraps onto another line.
    withScrollHeight(44);
    act(() => observers.forEach((run) => run()));
    expect(field.style.height).toBe("44px");
    vi.unstubAllGlobals();
  });
});
