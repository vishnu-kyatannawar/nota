// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { ITEM_ROW_ATTR, stealsFocus } from "./focus";

function mount(html: string): HTMLElement {
  document.body.innerHTML = html;
  const el = document.body.firstElementChild as HTMLElement;
  el.focus();
  return el;
}

afterEach(() => {
  document.body.innerHTML = "";
});

describe("stealsFocus", () => {
  it("is false when nothing is focused", () => {
    expect(stealsFocus()).toBe(false);
  });

  it("is true for a text input, such as the rename box or search", () => {
    mount(`<input type="text" />`);
    expect(stealsFocus()).toBe(true);
  });

  it("is true for a textarea", () => {
    mount(`<textarea></textarea>`);
    expect(stealsFocus()).toBe(true);
  });

  it("is true inside a contenteditable, such as the notes editor", () => {
    document.body.innerHTML = `<div contenteditable="true"><p id="inner">x</p></div>`;
    const inner = document.getElementById("inner") as HTMLElement;
    inner.tabIndex = 0;
    inner.focus();
    expect(stealsFocus()).toBe(true);
  });

  it("is false for an item row, which the list is allowed to move between", () => {
    mount(`<input type="text" ${ITEM_ROW_ATTR} />`);
    expect(stealsFocus()).toBe(false);
  });

  it("is false for a button or checkbox", () => {
    mount(`<button>x</button>`);
    expect(stealsFocus()).toBe(false);
    document.body.innerHTML = "";
    mount(`<input type="checkbox" />`);
    expect(stealsFocus()).toBe(false);
  });
});
