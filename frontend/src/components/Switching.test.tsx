// @vitest-environment jsdom
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("./CodeEditor", () => ({ CodeEditor: () => <div /> }));
vi.mock("./NotesEditor", () => ({ NotesEditor: () => <div /> }));

// Two workplans, each with its own items, so switching between them is real.
const byPath: Record<string, { id: string; text: string }[]> = {
  "Workplans/2026-09-03.md": [{ id: "a", text: "yesterday" }],
  "Workplans/2026-09-04.md": [{ id: "b", text: "today" }],
};

const noteFor = (p: string) => ({
  path: p, id: p, type: "workplan", date: p.slice(-13, -3), hours: "07:30", dayType: "work",
  layout: "both", labels: [], title: p, body: "",
  items: byPath[p].map((it) => ({
    kind: "", level: 0, id: it.id, text: it.text, done: false, depth: 0, body: null,
    createdAt: "09:00", doneAt: "", from: "", carried: 0, recurring: "",
  })),
});

vi.mock("../lib/api", async (orig) => {
  const real = await orig<typeof import("../lib/api")>();
  return {
    ...real,
    api: {
      note: async (p: string) => noteFor(p),
      saveItems: async (p: string, inputs: { id: string; text: string }[]) => {
        byPath[p] = inputs.map((it, i) => ({ id: it.id || `${p}-new${i}`, text: it.text }));
        return byPath[p].map((it) => ({ ID: it.id, CreatedAt: "09:00", DoneAt: "", From: "", Carried: 0, Recurring: "" }));
      },
      saveBody: async () => {},
    },
  };
});

const { NoteView } = await import("./NoteView");

const rows = () => screen.getAllByLabelText("Item text") as HTMLTextAreaElement[];

function show(path: string, reloadToken: number) {
  return (
    <NoteView path={path} dark={false} reloadToken={reloadToken} allLabels={[]} todayPath={null}
      split="rows" onSplit={() => {}} onShellChanged={() => {}} onError={() => {}} />
  );
}

async function settle(ms = 0) {
  await act(async () => {
    await Promise.resolve();
    if (ms) vi.advanceTimersByTime(ms);
    for (let i = 0; i < 6; i++) await Promise.resolve();
  });
}

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  byPath["Workplans/2026-09-03.md"] = [{ id: "a", text: "yesterday" }];
  byPath["Workplans/2026-09-04.md"] = [{ id: "b", text: "today" }];
});
afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("switching between workplans", () => {
  it("does not carry an empty row from one workplan into the next", async () => {
    // Switching flushes the old note and the watcher reports that write back,
    // so the note being opened is loaded twice at once. The second load must
    // not treat the previous workplan's rows as this one's.
    const { rerender } = render(show("Workplans/2026-09-03.md", 0));
    await settle();
    expect(rows().map((r) => r.value)).toEqual(["yesterday"]);

    fireEvent.click(screen.getByText("Add item"));
    await settle();
    expect(rows()).toHaveLength(2);

    // Switch, with the flush-save's echo arriving in the same commit.
    rerender(show("Workplans/2026-09-04.md", 1));
    await settle(600);

    expect(rows().map((r) => r.value)).toEqual(["today"]);
  });

  it("does not accumulate rows over repeated switching", async () => {
    const { rerender } = render(show("Workplans/2026-09-03.md", 0));
    await settle();
    fireEvent.click(screen.getByText("Add item"));
    await settle();

    for (let i = 1; i <= 6; i++) {
      const path = i % 2 ? "Workplans/2026-09-04.md" : "Workplans/2026-09-03.md";
      rerender(show(path, i));
      await settle(600);
    }
    // Back on yesterday: its one item, and at most the blank row it owns.
    expect(rows().length).toBeLessThanOrEqual(2);
    expect(rows()[0].value).toBe("yesterday");
  });
});
