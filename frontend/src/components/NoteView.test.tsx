// @vitest-environment jsdom
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// CodeMirror and Tiptap need a real browser; neither is under test here.
vi.mock("./CodeEditor", () => ({ CodeEditor: () => <div /> }));
vi.mock("./NotesEditor", () => ({ NotesEditor: () => <div data-testid="notes" /> }));

const serverItems: { id: string; text: string }[] = [];
let noteType = "note";
let noteLayout = "items";
const note = () => ({
  path: "Lists/a.md", id: "n1", type: noteType, date: "", hours: "", dayType: "", layout: noteLayout,
  labels: [], title: "a", body: "",
  items: serverItems.map((it, i) => ({
    kind: "", level: 0, id: it.id, text: it.text, done: false, depth: 0, body: null,
    createdAt: "09:0" + i, doneAt: "", from: "", carried: 0, recurring: "",
  })),
});

const saveItems = vi.fn(async (_p: string, inputs: { id: string; text: string }[]) => {
  // Go mints an id for anything new and returns metadata per saved row.
  serverItems.length = 0;
  return inputs.map((it, i) => {
    const id = it.id || `srv${i}`;
    serverItems.push({ id, text: it.text });
    return { ID: id, CreatedAt: "09:00", DoneAt: "", From: "", Carried: 0, Recurring: "" };
  });
});

vi.mock("../lib/api", async (orig) => {
  const real = await orig<typeof import("../lib/api")>();
  return {
    ...real,
    api: {
      note: vi.fn(async () => note()),
      saveItems: (p: string, i: never) => saveItems(p, i),
      saveBody: vi.fn(async () => {}),
    },
  };
});

const { NoteView } = await import("./NoteView");

const view = (reloadToken = 0, split: "rows" | "columns" = "rows", onSplit = () => {}) =>
  render(
    <NoteView path="Lists/a.md" dark={false} reloadToken={reloadToken} allLabels={[]} todayPath={null}
      split={split} onSplit={onSplit} onShellChanged={() => {}} onError={() => {}} />,
  );

const rows = () => screen.getAllByLabelText("Item text") as HTMLTextAreaElement[];

/** Puts the caret where it would be if you had just finished typing the row. */
function atEnd(el: HTMLTextAreaElement) {
  el.focus();
  el.setSelectionRange(el.value.length, el.value.length);
}

beforeEach(() => {
  noteType = "note";
  noteLayout = "items";
  vi.useFakeTimers({ shouldAdvanceTime: true });
  serverItems.length = 0;
  serverItems.push({ id: "a", text: "one" });
  saveItems.mockClear();
});
afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

// Lets the mocked promises settle and the debounced save fire.
async function settle(ms = 0) {
  await act(async () => {
    await Promise.resolve();
    if (ms) vi.advanceTimersByTime(ms);
    await Promise.resolve();
    await Promise.resolve();
  });
}

describe("adding an item", () => {
  it("keeps the new row after the save writes the file and the watcher reloads it", async () => {
    // The reported bug: the row shows for about a second, then vanishes, and
    // only typing fast enough to keep the note dirty rescues it.
    const { rerender } = view();
    await settle();
    expect(rows()).toHaveLength(1);

    atEnd(rows()[0]);
    fireEvent.keyDown(rows()[0], { key: "Enter" });
    await settle();
    expect(rows()).toHaveLength(2);

    // An empty row has nothing to write, so it does not provoke a save at all.
    await settle(500);
    expect(saveItems).not.toHaveBeenCalled();

    // A reload arrives anyway — an edit on disk, or another note being saved.
    rerender(
      <NoteView path="Lists/a.md" dark={false} reloadToken={1} allLabels={[]} todayPath={null}
        split="rows" onSplit={() => {}} onShellChanged={() => {}} onError={() => {}} />,
    );
    await settle();

    expect(rows()).toHaveLength(2);
    expect(rows()[1].value).toBe("");
  });

  it("keeps a row added between two items where it was put", async () => {
    serverItems.push({ id: "b", text: "two" });
    const { rerender } = view();
    await settle();
    expect(rows()).toHaveLength(2);

    atEnd(rows()[0]);
    fireEvent.keyDown(rows()[0], { key: "Enter" });
    await settle(500);

    rerender(
      <NoteView path="Lists/a.md" dark={false} reloadToken={1} allLabels={[]} todayPath={null}
        split="rows" onSplit={() => {}} onShellChanged={() => {}} onError={() => {}} />,
    );
    await settle();

    expect(rows().map((r) => r.value)).toEqual(["one", "", "two"]);
  });

  it("splits the row when Enter is pressed in the middle of it", async () => {
    serverItems.length = 0;
    serverItems.push({ id: "a", text: "hello world" });
    view();
    await settle();

    const input = rows()[0];
    input.focus();
    input.setSelectionRange(5, 5);
    fireEvent.keyDown(input, { key: "Enter" });
    await settle();

    expect(rows().map((r) => r.value)).toEqual(["hello", "world"]);
    // The caret lands at the start of the tail that just moved down.
    expect(document.activeElement).toBe(rows()[1]);
    expect(rows()[1].selectionStart).toBe(0);
  });

  it("pushes the row down when Enter is pressed at the start of it", async () => {
    serverItems.length = 0;
    serverItems.push({ id: "a", text: "one" });
    view();
    await settle();

    const input = rows()[0];
    input.focus();
    input.setSelectionRange(0, 0);
    fireEvent.keyDown(input, { key: "Enter" });
    await settle();

    expect(rows().map((r) => r.value)).toEqual(["", "one"]);
    // The caret stays with the text, which is now the second row.
    expect(document.activeElement).toBe(rows()[1]);
  });

  it("folds an item into the one above when Backspace is pressed at its start", async () => {
    serverItems.length = 0;
    serverItems.push({ id: "a", text: "hello" }, { id: "b", text: "world" });
    view();
    await settle();

    const second = rows()[1];
    second.focus();
    second.setSelectionRange(0, 0);
    fireEvent.keyDown(second, { key: "Backspace" });
    await settle();

    expect(rows().map((r) => r.value)).toEqual(["helloworld"]);
    expect(rows()[0].selectionStart).toBe(5);
  });

  it("leaves the first item alone when Backspace is pressed at its start", async () => {
    serverItems.length = 0;
    serverItems.push({ id: "a", text: "only" });
    view();
    await settle();

    const first = rows()[0];
    first.focus();
    first.setSelectionRange(0, 0);
    fireEvent.keyDown(first, { key: "Backspace" });
    await settle();

    expect(rows().map((r) => r.value)).toEqual(["only"]);
  });

  it("shows a long item on more than one line", async () => {
    serverItems.length = 0;
    serverItems.push({ id: "a", text: "a".repeat(400) });
    view();
    await settle();

    // A single-line input hid everything past the first line; a wrapping field
    // is what makes a long item readable.
    const field = rows()[0];
    expect(field.tagName).toBe("TEXTAREA");
    expect(getComputedStyle(field).overflow).not.toBe("scroll");
  });

  it("still saves the row once it has been typed into", async () => {
    view();
    await settle();
    atEnd(rows()[0]);
    fireEvent.keyDown(rows()[0], { key: "Enter" });
    await settle();
    fireEvent.change(rows()[1], { target: { value: "two" } });
    await settle(500);

    expect(serverItems.map((i) => i.text)).toEqual(["one", "two"]);
  });
});

describe("arranging items and notes", () => {
  const arrangement = () => screen.queryByRole("radiogroup", { name: "Arrangement" });

  it("offers the choice on the page itself, not buried in settings", async () => {
    noteLayout = "both";
    view();
    await settle();
    expect(arrangement()).not.toBeNull();
    expect(screen.getByRole("radio", { name: /Side by side/ })).toBeTruthy();
  });

  it("offers it on a workplan too, which always shows both", async () => {
    noteType = "workplan";
    noteLayout = "items";
    view();
    await settle();
    expect(arrangement()).not.toBeNull();
  });

  it("does not offer it when there is only one section on screen", async () => {
    noteLayout = "items";
    view();
    await settle();
    expect(arrangement()).toBeNull();
  });

  it("reports the choice so it can be remembered", async () => {
    noteLayout = "both";
    const onSplit = vi.fn();
    view(0, "rows", onSplit);
    await settle();
    fireEvent.click(screen.getByRole("radio", { name: /Side by side/ }));
    expect(onSplit).toHaveBeenCalledWith("columns");
  });

  it("puts the two sections in a row when side by side is chosen", async () => {
    noteLayout = "both";
    const { container } = view(0, "columns");
    await settle();
    expect(screen.getByRole("radio", { name: /Side by side/ }).getAttribute("aria-checked")).toBe("true");
    expect(container.querySelector(".lg\\:flex-row")).not.toBeNull();
  });

  it("stacks them by default", async () => {
    noteLayout = "both";
    const { container } = view(0, "rows");
    await settle();
    expect(container.querySelector(".lg\\:flex-row")).toBeNull();
  });
});

describe("adding and deleting rows", () => {
  it("adds exactly one row per click of Add item", async () => {
    serverItems.length = 0;
    serverItems.push({ id: "a", text: "one" });
    const { rerender } = view();
    await settle();
    expect(rows()).toHaveLength(1);

    fireEvent.click(screen.getByText("Add item"));
    await settle();
    expect(rows()).toHaveLength(2);

    // The save fires, the file is written, the watcher reports it back.
    await settle(500);
    rerender(
      <NoteView path="Lists/a.md" dark={false} reloadToken={1} allLabels={[]} todayPath={null}
        split="rows" onSplit={() => {}} onShellChanged={() => {}} onError={() => {}} />,
    );
    await settle();
    expect(rows()).toHaveLength(2);

    fireEvent.click(screen.getByText("Add item"));
    await settle(500);
    rerender(
      <NoteView path="Lists/a.md" dark={false} reloadToken={2} allLabels={[]} todayPath={null}
        split="rows" onSplit={() => {}} onShellChanged={() => {}} onError={() => {}} />,
    );
    await settle();
    expect(rows()).toHaveLength(3);
  });

  it("deletes a row without leaving more behind than it removed", async () => {
    serverItems.length = 0;
    serverItems.push({ id: "a", text: "one" }, { id: "b", text: "two" });
    const { rerender } = view();
    await settle();
    expect(rows()).toHaveLength(2);

    const del = screen.getAllByLabelText(/Delete item/)[1];
    fireEvent.click(del);
    await settle(500);
    rerender(
      <NoteView path="Lists/a.md" dark={false} reloadToken={1} allLabels={[]} todayPath={null}
        split="rows" onSplit={() => {}} onShellChanged={() => {}} onError={() => {}} />,
    );
    await settle();
    expect(rows().map((r) => r.value)).toEqual(["one"]);
  });

  it("lets an empty row be deleted", async () => {
    serverItems.length = 0;
    serverItems.push({ id: "a", text: "one" });
    const { rerender } = view();
    await settle();

    fireEvent.click(screen.getByText("Add item"));
    await settle(500);
    rerender(
      <NoteView path="Lists/a.md" dark={false} reloadToken={1} allLabels={[]} todayPath={null}
        split="rows" onSplit={() => {}} onShellChanged={() => {}} onError={() => {}} />,
    );
    await settle();
    expect(rows()).toHaveLength(2);

    fireEvent.click(screen.getAllByLabelText(/Delete item/)[1]);
    await settle(500);
    rerender(
      <NoteView path="Lists/a.md" dark={false} reloadToken={2} allLabels={[]} todayPath={null}
        split="rows" onSplit={() => {}} onShellChanged={() => {}} onError={() => {}} />,
    );
    await settle();
    expect(rows().map((r) => r.value)).toEqual(["one"]);
  });
});

describe("repeated reloads", () => {
  it("does not multiply the empty rows on every reload", async () => {
    serverItems.length = 0;
    serverItems.push({ id: "a", text: "one" });
    const { rerender } = view();
    await settle();

    fireEvent.click(screen.getByText("Add item"));
    fireEvent.click(screen.getByText("Add item"));
    await settle(500);
    expect(rows()).toHaveLength(3);

    for (let token = 1; token <= 5; token++) {
      rerender(
        <NoteView path="Lists/a.md" dark={false} reloadToken={token} allLabels={[]} todayPath={null}
          split="rows" onSplit={() => {}} onShellChanged={() => {}} onError={() => {}} />,
      );
      await settle(500);
    }
    expect(rows()).toHaveLength(3);
  });

  it("does not multiply a row whose text is only spaces", async () => {
    // A save skips such a row, so it is never in the file — but it looks like
    // an ordinary item on screen.
    serverItems.length = 0;
    serverItems.push({ id: "a", text: "one" }, { id: "b", text: "   " });
    const { rerender } = view();
    await settle();
    expect(rows()).toHaveLength(2);

    for (let token = 1; token <= 4; token++) {
      rerender(
        <NoteView path="Lists/a.md" dark={false} reloadToken={token} allLabels={[]} todayPath={null}
          split="rows" onSplit={() => {}} onShellChanged={() => {}} onError={() => {}} />,
      );
      await settle(500);
    }
    expect(rows()).toHaveLength(2);
  });
});
