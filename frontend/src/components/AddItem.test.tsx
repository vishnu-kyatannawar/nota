// @vitest-environment jsdom
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("./CodeEditor", () => ({ CodeEditor: () => <div /> }));
vi.mock("./NotesEditor", () => ({ NotesEditor: () => <div /> }));

// The real event bus, so a save's watcher echo reaches the app the way it does
// in the running application.
const listeners: Record<string, ((e: { data: unknown }) => void)[]> = {};
vi.mock("@wailsio/runtime", () => ({
  Browser: { OpenURL: vi.fn() },
  Dialogs: {},
  Events: {
    On: (name: string, cb: (e: { data: unknown }) => void) => {
      (listeners[name] ??= []).push(cb);
    },
    Off: (name: string) => { listeners[name] = []; },
  },
}));

let items: { id: string; text: string }[] = [];
const note = () => ({
  path: "Workplans/2026-09-04.md", id: "n1", type: "workplan", date: "2026-09-04",
  hours: "07:30", dayType: "work", layout: "both", labels: [], title: "2026-09-04", body: "",
  items: items.map((it) => ({
    kind: "", level: 0, id: it.id, text: it.text, done: false, depth: 0, body: null,
    createdAt: "09:00", doneAt: "", from: "", carried: 0, recurring: "",
  })),
});

const saveItems = vi.fn(async (_p: string, inputs: { id: string; text: string }[]) => {
  items = inputs.map((it, i) => ({ id: it.id || `srv${i}`, text: it.text }));
  // Every write is seen by the watcher and reported back.
  queueMicrotask(() => listeners["note:changed"]?.forEach((cb) => cb({ data: "Workplans/2026-09-04.md" })));
  return items.map((it) => ({ ID: it.id, CreatedAt: "09:00", DoneAt: "", From: "", Carried: 0, Recurring: "" }));
});

vi.mock("../lib/api", async (orig) => {
  const real = await orig<typeof import("../lib/api")>();
  return {
    ...real,
    api: {
      info: async () => ({
        version: "4.5.2", vaultPath: "/v", workplanDir: "/v/Workplans", theme: "light",
        fonts: { ui: "inter", notes: "inter", code: "jetbrains-mono", size: "m" },
        repository: "", website: "", releases: "", licence: "", updateCheck: "never", split: "rows",
      }),
      ensureToday: async () => "Workplans/2026-09-04.md",
      tree: async () => ({ name: "v", path: "", isFolder: true, children: [] }),
      workplans: async () => [],
      labels: async () => [],
      hoursThisWeek: async () => ({ from: "", to: "", minutes: 0, hours: "00:00" }),
      listTrash: async () => [],
      note: async () => note(),
      saveItems: (p: string, i: never) => saveItems(p, i),
      saveBody: async () => {},
      setTheme: async () => {},
      setFonts: async () => {},
      setSplit: async () => {},
      setUpdateCheck: async () => {},
      updateState: async () => ({ phase: "idle", version: "", percent: 0, message: "", canInstall: false, releaseUrl: "" }),
    },
  };
});

const App = (await import("../App")).default;

const rows = () => screen.queryAllByLabelText("Item text") as HTMLTextAreaElement[];

async function settle(ms = 0) {
  await act(async () => {
    await Promise.resolve();
    if (ms) vi.advanceTimersByTime(ms);
    for (let i = 0; i < 6; i++) await Promise.resolve();
  });
}

beforeEach(() => {
  vi.stubGlobal("matchMedia", () => ({
    matches: false, addEventListener: () => {}, removeEventListener: () => {},
  }));
  vi.useFakeTimers({ shouldAdvanceTime: true });
  items = [{ id: "a", text: "one" }];
  for (const k of Object.keys(listeners)) listeners[k] = [];
  saveItems.mockClear();
});
afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("Add item, with the watcher reporting every save back", () => {
  it("adds exactly one empty row", async () => {
    render(<App />);
    await settle();
    await settle(100);
    expect(rows().map((r) => r.value)).toEqual(["one"]);

    fireEvent.click(screen.getByText("Add item"));
    await settle();
    expect(rows()).toHaveLength(2);

    // The save runs, the watcher reports it, the note reloads.
    await settle(600);
    await settle(600);
    expect(rows().map((r) => r.value)).toEqual(["one", ""]);
  });

  it("keeps a new empty row below the row just typed, not above it", async () => {
    render(<App />);
    await settle();
    await settle(100);

    fireEvent.click(screen.getByText("Add item"));
    await settle();
    fireEvent.change(rows()[1], { target: { value: "two" } });
    await settle(600);
    await settle(600);

    fireEvent.click(screen.getByText("Add item"));
    await settle();
    await settle(600);
    await settle(600);
    expect(rows().map((r) => r.value)).toEqual(["one", "two", ""]);
  });

  it("deleting an empty row removes it and adds nothing", async () => {
    render(<App />);
    await settle();
    await settle(100);

    fireEvent.click(screen.getByText("Add item"));
    await settle(600);
    await settle(600);
    expect(rows()).toHaveLength(2);

    fireEvent.click(screen.getAllByLabelText(/Delete item/)[1]);
    await settle(600);
    await settle(600);
    expect(rows().map((r) => r.value)).toEqual(["one"]);
  });

  it("does not accumulate rows over repeated adds and deletes", async () => {
    render(<App />);
    await settle();
    await settle(100);

    for (let i = 0; i < 4; i++) {
      fireEvent.click(screen.getByText("Add item"));
      await settle(600);
      await settle(600);
      const last = screen.getAllByLabelText(/Delete item/).length - 1;
      fireEvent.click(screen.getAllByLabelText(/Delete item/)[last]);
      await settle(600);
      await settle(600);
    }
    expect(rows().map((r) => r.value)).toEqual(["one"]);
  });
});
