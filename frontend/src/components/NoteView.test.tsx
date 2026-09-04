// @vitest-environment jsdom
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// CodeMirror and Tiptap need a real browser; neither is under test here.
vi.mock("./CodeEditor", () => ({ CodeEditor: () => <div /> }));
vi.mock("./NotesEditor", () => ({ NotesEditor: () => <div data-testid="notes" /> }));

const serverItems: { id: string; text: string }[] = [];
const note = () => ({
  path: "Lists/a.md", id: "n1", type: "note", date: "", hours: "", dayType: "", layout: "items",
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

const view = (reloadToken = 0) =>
  render(
    <NoteView path="Lists/a.md" dark={false} reloadToken={reloadToken} allLabels={[]} todayPath={null}
      onShellChanged={() => {}} onError={() => {}} />,
  );

const rows = () => screen.getAllByLabelText("Item text") as HTMLInputElement[];

beforeEach(() => {
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

    fireEvent.keyDown(rows()[0], { key: "Enter" });
    await settle();
    expect(rows()).toHaveLength(2);

    // The debounced save runs and writes the file.
    await settle(500);
    expect(saveItems).toHaveBeenCalledOnce();

    // Go's watcher sees our own write and the parent bumps reloadToken.
    rerender(
      <NoteView path="Lists/a.md" dark={false} reloadToken={1} allLabels={[]} todayPath={null}
        onShellChanged={() => {}} onError={() => {}} />,
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

    fireEvent.keyDown(rows()[0], { key: "Enter" });
    await settle(500);

    rerender(
      <NoteView path="Lists/a.md" dark={false} reloadToken={1} allLabels={[]} todayPath={null}
        onShellChanged={() => {}} onError={() => {}} />,
    );
    await settle();

    expect(rows().map((r) => r.value)).toEqual(["one", "", "two"]);
  });

  it("still saves the row once it has been typed into", async () => {
    view();
    await settle();
    fireEvent.keyDown(rows()[0], { key: "Enter" });
    await settle();
    fireEvent.change(rows()[1], { target: { value: "two" } });
    await settle(500);

    expect(serverItems.map((i) => i.text)).toEqual(["one", "two"]);
  });
});
