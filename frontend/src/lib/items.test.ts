import { describe, expect, it } from "vitest";
import { blankItem, headingShortcut, initialState, keepUnsaved, mergeSaved, reducer, splitPastedList, toInputs, type LocalItem } from "./items";

const item = (text: string, depth = 0, extra: Partial<LocalItem> = {}): LocalItem => ({
  ...blankItem(depth, text),
  id: `id-${text}`,
  ...extra,
});

const seeded = () => ({
  ...initialState,
  items: [item("one"), item("two"), item("three")],
});

describe("Enter", () => {
  it("inserts a blank row below at the same depth and focuses it", () => {
    const s = reducer({ ...seeded(), items: [item("a", 1)] }, { type: "insertAfter", index: 0 });
    expect(s.items.map((i) => i.text)).toEqual(["a", ""]);
    expect(s.items[1].depth).toBe(1);
    expect(s.focus).toBe(1);
    expect(s.dirty).toBe(true);
  });
});

describe("arrow keys", () => {
  it("move focus between rows and stop at the ends", () => {
    let s = reducer(seeded(), { type: "focus", index: 0 });
    s = reducer(s, { type: "move", index: 0, delta: 1 });
    expect(s.focus).toBe(1);
    s = reducer(s, { type: "move", index: 1, delta: -1 });
    expect(s.focus).toBe(0);
    s = reducer(s, { type: "move", index: 0, delta: -1 });
    expect(s.focus).toBe(0);
    s = reducer(s, { type: "move", index: 2, delta: 1 });
    expect(s.focus).toBe(2);
  });
});

describe("Backspace on an empty row", () => {
  it("removes it and focuses the row above", () => {
    const s = reducer({ ...seeded(), items: [item("one"), item("", 0)] }, { type: "remove", index: 1 });
    expect(s.items.map((i) => i.text)).toEqual(["one"]);
    expect(s.focus).toBe(0);
  });

  it("out-dents the children of a removed parent", () => {
    const s = reducer(
      { ...initialState, items: [item("parent"), item("child", 1), item("grandchild", 2), item("sibling")] },
      { type: "remove", index: 0 },
    );
    expect(s.items.map((i) => [i.text, i.depth])).toEqual([["child", 0], ["grandchild", 1], ["sibling", 0]]);
  });

  it("leaves no focus when the last row goes", () => {
    const s = reducer({ ...initialState, items: [item("")] }, { type: "remove", index: 0 });
    expect(s.items).toEqual([]);
    expect(s.focus).toBeNull();
  });
});

describe("Tab", () => {
  it("cannot nest deeper than one level below the row above", () => {
    let s = reducer(seeded(), { type: "indent", index: 1, delta: 1 });
    expect(s.items[1].depth).toBe(1);
    s = reducer(s, { type: "indent", index: 1, delta: 1 });
    expect(s.items[1].depth).toBe(1);
  });

  it("cannot indent the first row", () => {
    const s = reducer(seeded(), { type: "indent", index: 0, delta: 1 });
    expect(s.items[0].depth).toBe(0);
  });

  it("moves descendants with their parent", () => {
    const s = reducer(
      { ...initialState, items: [item("a"), item("b"), item("c", 1)] },
      { type: "indent", index: 1, delta: 1 },
    );
    expect(s.items.map((i) => i.depth)).toEqual([0, 1, 2]);
  });
});

describe("paste", () => {
  it("splits bullets, numbers, checkboxes and indentation", () => {
    const lines = splitPastedList("- first\n* second\n• third\n1. fourth\n- [ ] fifth\n- [x] sixth\n\n  - child\nplain\n");
    expect(lines).toEqual([
      { text: "first", done: false, depth: 0 },
      { text: "second", done: false, depth: 0 },
      { text: "third", done: false, depth: 0 },
      { text: "fourth", done: false, depth: 0 },
      { text: "fifth", done: false, depth: 0 },
      { text: "sixth", done: true, depth: 0 },
      { text: "child", done: false, depth: 1 },
      { text: "plain", done: false, depth: 0 },
    ]);
  });

  it("handles Windows line endings", () => {
    expect(splitPastedList("a\r\nb\r\n").map((l) => l.text)).toEqual(["a", "b"]);
  });

  it("replaces an empty row and inserts below a filled one", () => {
    const lines = splitPastedList("- x\n- y");
    let s = reducer({ ...initialState, items: [item("")] }, { type: "insertMany", index: 0, lines });
    expect(s.items.map((i) => i.text)).toEqual(["x", "y"]);
    expect(s.focus).toBe(1);

    s = reducer({ ...initialState, items: [item("kept")] }, { type: "insertMany", index: 0, lines });
    expect(s.items.map((i) => i.text)).toEqual(["kept", "x", "y"]);
    expect(s.focus).toBe(2);
  });

  it("keeps done state from [x] lines", () => {
    const s = reducer({ ...initialState, items: [item("")] }, { type: "insertMany", index: 0, lines: splitPastedList("- [x] done\n- open") });
    expect(s.items.map((i) => i.done)).toEqual([true, false]);
  });
});

describe("save round trip", () => {
  it("leaves out blank rows and remembers where the sent ones came from", () => {
    const { inputs, kept } = toInputs([item("a"), item(""), item("b", 1), { ...item(""), body: ["has notes"] }]);
    expect(inputs.map((i) => i.text)).toEqual(["a", "b", ""]);
    expect(kept).toEqual([0, 2, 3]);
  });

  it("adopts minted ids but keeps text typed since the save began", () => {
    const local = [item("a"), item("typing more", 0, { id: "" })];
    const merged = mergeSaved(local, [
      { id: "id-a", createdAt: "08:00", doneAt: "", from: "", carried: 0, recurring: "" },
      { id: "MINTED", createdAt: "09:00", doneAt: "", from: "", carried: 0, recurring: "" },
    ], [0, 1]);
    expect(merged[1].id).toBe("MINTED");
    expect(merged[1].text).toBe("typing more");
    expect(merged[1].createdAt).toBe("09:00");
    expect(merged[1].key).toBe(local[1].key);
  });

  it("ignores rows that were removed while the save was in flight", () => {
    const merged = mergeSaved([item("only")], [
      { id: "x", createdAt: "", doneAt: "", from: "", carried: 0, recurring: "" },
      { id: "gone", createdAt: "", doneAt: "", from: "", carried: 0, recurring: "" },
    ], [0, 5]);
    expect(merged).toHaveLength(1);
    expect(merged[0].id).toBe("x");
  });
});

describe("headings", () => {
  it("turns a row into a heading and back, resetting what headings cannot have", () => {
    let s = reducer({ ...initialState, items: [item("Must", 1, { done: true, body: ["x"] })] }, { type: "setKind", index: 0, kind: "heading" });
    expect(s.items[0]).toMatchObject({ kind: "heading", level: 2, depth: 0, done: false, body: [] });
    s = reducer(s, { type: "setKind", index: 0, kind: "" });
    expect(s.items[0]).toMatchObject({ kind: "", level: 0 });
  });

  it("cannot be toggled or indented", () => {
    const heading = { ...blankItem(0, "Must", "heading") };
    let s = reducer({ ...initialState, items: [item("a"), heading] }, { type: "toggle", index: 1 });
    expect(s.items[1].done).toBe(false);
    s = reducer(s, { type: "indent", index: 1, delta: 1 });
    expect(s.items[1].depth).toBe(0);
  });

  it("Enter under a heading inserts a top-level item", () => {
    const s = reducer({ ...initialState, items: [blankItem(0, "Must", "heading")] }, { type: "insertAfter", index: 0 });
    expect(s.items[1]).toMatchObject({ kind: "", depth: 0 });
    expect(s.focus).toBe(1);
  });

  it("is sent to the backend with its kind and level", () => {
    const { inputs } = toInputs([{ ...blankItem(0, "Later", "heading"), level: 3 }, item("a")]);
    expect(inputs[0]).toMatchObject({ kind: "heading", level: 3, text: "Later" });
    expect(inputs[1].kind).toBe("");
  });

  it("recognises the ## shortcut", () => {
    expect(headingShortcut("## Must")).toEqual({ level: 2, rest: "Must" });
    expect(headingShortcut("### ")).toEqual({ level: 3, rest: "" });
    expect(headingShortcut("#notalabel")).toBeNull();
    expect(headingShortcut("plain")).toBeNull();
  });
});

describe("keepUnsaved", () => {
  const saved = (id: string, text: string): LocalItem => ({
    ...blankItem(0, text), id, key: id,
  });
  const blank = (key = "new"): LocalItem => ({ ...blankItem(), key });

  it("keeps a row added at the end", () => {
    // The reported bug: press Enter, the row shows for a second, then the save
    // triggers a reload and the row is gone.
    const local = [saved("a", "one"), blank()];
    const got = keepUnsaved(local, [saved("a", "one")]);
    expect(got.map((i) => i.id)).toEqual(["a", ""]);
    expect(got[1]).toBe(local[1]);
  });

  it("keeps a row added in the middle, in the middle", () => {
    const local = [saved("a", "one"), blank(), saved("b", "two")];
    const got = keepUnsaved(local, [saved("a", "one"), saved("b", "two")]);
    expect(got.map((i) => i.id)).toEqual(["a", "", "b"]);
  });

  it("keeps a row added before everything", () => {
    const local = [blank(), saved("a", "one")];
    const got = keepUnsaved(local, [saved("a", "one")]);
    expect(got.map((i) => i.id)).toEqual(["", "a"]);
  });

  it("keeps several rows, each where it was", () => {
    const local = [blank("x"), saved("a", "one"), blank("y"), saved("b", "two"), blank("z")];
    const got = keepUnsaved(local, [saved("a", "one"), saved("b", "two")]);
    expect(got.map((i) => i.key)).toEqual(["x", "a", "y", "b", "z"]);
  });

  it("does not lose a row whose neighbour was deleted elsewhere", () => {
    const local = [saved("a", "one"), blank("x")];
    const got = keepUnsaved(local, [saved("b", "two")]);
    expect(got.map((i) => i.key)).toEqual(["b", "x"]);
  });

  it("returns the incoming list untouched when nothing is pending", () => {
    const incoming = [saved("a", "one")];
    expect(keepUnsaved([saved("a", "one")], incoming)).toBe(incoming);
  });

  it("keeps a row the user has cleared and is retyping", () => {
    // toInputs skips it too, so the save deletes it and the reload would drop it.
    const cleared: LocalItem = { ...saved("a", ""), text: "" };
    const got = keepUnsaved([cleared, saved("b", "two")], [saved("b", "two")]);
    expect(got.map((i) => i.key)).toEqual(["a", "b"]);
  });

  it("keeps exactly the rows a save would skip", () => {
    const local = [saved("a", "one"), blank("x"), saved("b", "two")];
    const { kept } = toInputs(local);
    // The rows toInputs drops are the rows keepUnsaved must put back.
    expect(kept).toEqual([0, 2]);
    expect(keepUnsaved(local, [saved("a", "one"), saved("b", "two")]).length).toBe(3);
  });
});
