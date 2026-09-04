// @vitest-environment jsdom
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("./CodeEditor", () => ({ CodeEditor: () => <div /> }));
vi.mock("./NotesEditor", () => ({ NotesEditor: () => <div /> }));

let items: { id: string; text: string; recurring: string }[] = [];
let noteType = "workplan";

const addRepeating = vi.fn(async (text: string) => {
  items = [{ id: `r${items.length}`, text, recurring: "tpl-1" }, ...items];
});
const stopRepeating = vi.fn(async (id: string) => {
  items = items.filter((it) => it.recurring !== id);
});

vi.mock("../lib/api", async (orig) => {
  const real = await orig<typeof import("../lib/api")>();
  return {
    ...real,
    api: {
      note: async () => ({
        path: "Workplans/2026-09-04.md", id: "n1", type: noteType, date: "2026-09-04",
        hours: "07:30", dayType: "work", layout: "both", labels: [], title: "t", body: "",
        items: items.map((it) => ({
          kind: "", level: 0, id: it.id, text: it.text, done: false, depth: 0, body: null,
          createdAt: "09:00", doneAt: "", from: "", carried: 0, recurring: it.recurring,
        })),
      }),
      saveItems: async () => [],
      saveBody: async () => {},
      addRepeating: (t: string) => addRepeating(t),
      stopRepeating: (id: string) => stopRepeating(id),
    },
  };
});

const { NoteView } = await import("./NoteView");

beforeAll(() => {
  HTMLDialogElement.prototype.showModal = function () { this.open = true; };
  HTMLDialogElement.prototype.close = function () { this.open = false; };
});

const view = () =>
  render(
    <NoteView path="Workplans/2026-09-04.md" dark={false} reloadToken={0} allLabels={[]} todayPath="Workplans/2026-09-04.md"
      split="rows" onSplit={() => {}} onShellChanged={() => {}} onError={() => {}} />,
  );

const rows = () => screen.getAllByLabelText("Item text") as HTMLTextAreaElement[];

async function settle() {
  await act(async () => { for (let i = 0; i < 6; i++) await Promise.resolve(); });
}

beforeEach(() => {
  noteType = "workplan";
  items = [
    { id: "r1", text: "Check email", recurring: "tpl-1" },
    { id: "a1", text: "Ship the thing", recurring: "" },
  ];
  addRepeating.mockClear();
  stopRepeating.mockClear();
});
afterEach(cleanup);

describe("the repeats-daily section", () => {
  it("puts what repeats above the day's own work", async () => {
    view();
    await settle();
    expect(screen.getByText("Repeats daily")).toBeTruthy();
    expect(screen.getByText("Today")).toBeTruthy();
    expect(rows().map((r) => r.value)).toEqual(["Check email", "Ship the thing"]);
  });

  it("groups by the marker, not by where the row happens to sit", async () => {
    items = [
      { id: "a1", text: "Ship the thing", recurring: "" },
      { id: "r1", text: "Check email", recurring: "tpl-1" },
    ];
    view();
    await settle();
    // The repeating one is lifted to the top group even though the file has it second.
    expect(rows().map((r) => r.value)).toEqual(["Check email", "Ship the thing"]);
  });

  it("adds a repeating item from the workplan itself", async () => {
    view();
    await settle();
    const field = screen.getByLabelText("Add an item that repeats every day");
    fireEvent.change(field, { target: { value: "Check calendar" } });
    fireEvent.keyDown(field, { key: "Enter" });
    await settle();

    expect(addRepeating).toHaveBeenCalledWith("Check calendar");
    await settle();
    expect(rows()[0].value).toBe("Check calendar");
  });

  it("does not add an empty one", async () => {
    view();
    await settle();
    const field = screen.getByLabelText("Add an item that repeats every day");
    fireEvent.change(field, { target: { value: "   " } });
    fireEvent.keyDown(field, { key: "Enter" });
    await settle();
    expect(addRepeating).not.toHaveBeenCalled();
  });

  it("asks before stopping one, and says history is kept", async () => {
    view();
    await settle();
    fireEvent.click(screen.getAllByLabelText("More")[0]);
    fireEvent.click(screen.getByText("Stop repeating…"));
    await settle();

    expect(document.body.textContent).toContain("Workplans already written keep it");
    expect(stopRepeating).not.toHaveBeenCalled();

    fireEvent.click(screen.getByText("Stop repeating"));
    await settle();
    expect(stopRepeating).toHaveBeenCalledWith("tpl-1");
  });

  it("leaves an ordinary page alone", async () => {
    noteType = "note";
    items = [{ id: "a1", text: "Ship the thing", recurring: "" }];
    view();
    await settle();
    expect(screen.queryByText("Repeats daily")).toBeNull();
    expect(screen.queryByLabelText("Add an item that repeats every day")).toBeNull();
  });
});
