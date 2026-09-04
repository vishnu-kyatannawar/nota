// @vitest-environment jsdom
import { cleanup, render } from "@testing-library/react";
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
