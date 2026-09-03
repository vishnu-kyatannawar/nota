/**
 * The editor's local model of a note's action items.
 *
 * Every keystroke updates this state immediately; a debounced save sends the
 * whole list to Go. Keeping the list local is what makes Enter, the arrow keys
 * and multi-line paste feel instant — none of them wait on a round trip.
 *
 * Everything here is pure so it can be unit-tested without React.
 */

export type ItemKind = "" | "heading";

export type LocalItem = {
  /** "" for an action item, "heading" for a group heading between items. */
  kind: ItemKind;
  /** Heading level for headings (new ones are 2); ignored for items. */
  level: number;
  /** Empty for an item typed since the last save; Go mints the id on save. */
  id: string;
  text: string;
  done: boolean;
  depth: number;
  body: string[];
  /** Display-only metadata that Go owns; never sent back. */
  createdAt?: string;
  doneAt?: string;
  from?: string;
  carried?: number;
  recurring?: string;
  /** Stable React key that survives the id being minted. */
  key: string;
};

export type ItemsState = {
  items: LocalItem[];
  /** Index of the row that should own keyboard focus, or null. */
  focus: number | null;
  /** Where the caret should land when focus moves: at the end, or a column. */
  caret: "end" | number;
  /** True once the user changed something that has not been saved yet. */
  dirty: boolean;
};

export type ItemsAction =
  | { type: "replaceAll"; items: LocalItem[]; dirty?: boolean }
  | { type: "setText"; index: number; text: string }
  | { type: "toggle"; index: number }
  | { type: "setBody"; index: number; body: string[] }
  | { type: "insertAfter"; index: number; text?: string; kind?: ItemKind }
  | { type: "setKind"; index: number; kind: ItemKind }
  | { type: "remove"; index: number }
  | { type: "indent"; index: number; delta: 1 | -1 }
  | { type: "move"; index: number; delta: 1 | -1 }
  | { type: "focus"; index: number | null; caret?: "end" | number }
  | { type: "insertMany"; index: number; lines: PastedLine[] };

export const MAX_DEPTH = 6;

let keyCounter = 0;
export function newKey(): string {
  keyCounter += 1;
  return `k${Date.now().toString(36)}${keyCounter}`;
}

export function blankItem(depth = 0, text = "", kind: ItemKind = ""): LocalItem {
  return { kind, level: kind === "heading" ? 2 : 0, id: "", text, done: false, depth: kind === "heading" ? 0 : depth, body: [], key: newKey() };
}

export const initialState: ItemsState = { items: [], focus: null, caret: "end", dirty: false };

function clampDepth(depth: number, above: LocalItem | undefined): number {
  // An item can nest at most one level deeper than the row above it.
  const limit = above ? Math.min(MAX_DEPTH, above.depth + 1) : 0;
  return Math.max(0, Math.min(depth, limit));
}

export function reducer(state: ItemsState, action: ItemsAction): ItemsState {
  const items = state.items;

  switch (action.type) {
    case "replaceAll":
      return { ...state, items: action.items, dirty: action.dirty ?? false };

    case "setText": {
      const next = items.slice();
      next[action.index] = { ...next[action.index], text: action.text };
      return { ...state, items: next, dirty: true };
    }

    case "toggle": {
      if (items[action.index]?.kind === "heading") return state;
      const next = items.slice();
      next[action.index] = { ...next[action.index], done: !next[action.index].done };
      return { ...state, items: next, dirty: true };
    }

    case "setBody": {
      const next = items.slice();
      next[action.index] = { ...next[action.index], body: action.body };
      return { ...state, items: next, dirty: true };
    }

    case "insertAfter": {
      const at = action.index;
      // Under a heading the next row is a top-level item; otherwise it matches the row above.
      const depth = items[at]?.kind === "heading" ? 0 : (items[at]?.depth ?? 0);
      const next = items.slice();
      next.splice(at + 1, 0, blankItem(depth, action.text ?? "", action.kind ?? ""));
      return { items: next, focus: at + 1, caret: "end", dirty: true };
    }

    case "setKind": {
      const it = items[action.index];
      if (!it || it.kind === action.kind) return state;
      const next = items.slice();
      next[action.index] =
        action.kind === "heading"
          ? { ...it, kind: "heading", level: 2, depth: 0, done: false, body: [] }
          : { ...it, kind: "", level: 0 };
      return { ...state, items: next, dirty: true };
    }

    case "remove": {
      if (items.length === 0) return state;
      const next = items.slice();
      next.splice(action.index, 1);
      // Children of a removed parent step out one level so they keep a valid parent.
      for (let i = action.index; i < next.length && next[i].depth > (items[action.index]?.depth ?? 0); i++) {
        next[i] = { ...next[i], depth: next[i].depth - 1 };
      }
      const focus = next.length === 0 ? null : Math.max(0, action.index - 1);
      return { items: next, focus, caret: "end", dirty: true };
    }

    case "indent": {
      const next = items.slice();
      const it = next[action.index];
      if (it.kind === "heading") return state;
      const depth = clampDepth(it.depth + action.delta, next[action.index - 1]);
      if (depth === it.depth) return state;
      next[action.index] = { ...it, depth };
      // Descendants move with their parent.
      for (let i = action.index + 1; i < next.length && next[i].depth > it.depth; i++) {
        next[i] = { ...next[i], depth: Math.max(0, next[i].depth + action.delta) };
      }
      return { ...state, items: next, dirty: true };
    }

    case "move": {
      if (items.length === 0) return state;
      // Past either end, focus stays on the row that pressed the key.
      const target = Math.max(0, Math.min(items.length - 1, action.index + action.delta));
      return { ...state, focus: target, caret: "end" };
    }

    case "focus":
      return { ...state, focus: action.index, caret: action.caret ?? "end" };

    case "insertMany": {
      if (action.lines.length === 0) return state;
      const at = action.index;
      const base = items[at]?.depth ?? 0;
      const current = items[at];
      const next = items.slice();

      // Pasting into an empty row replaces it; into a filled row inserts below.
      const replaceCurrent = current !== undefined && current.text.trim() === "";
      const inserted = action.lines.map((l) => ({
        ...blankItem(Math.min(MAX_DEPTH, base + l.depth), l.text),
        done: l.done,
      }));
      if (replaceCurrent) next.splice(at, 1, ...inserted);
      else next.splice(at + 1, 0, ...inserted);

      const last = (replaceCurrent ? at : at + 1) + inserted.length - 1;
      return { items: next, focus: last, caret: "end", dirty: true };
    }
  }
}

export type PastedLine = { text: string; done: boolean; depth: number };

const BULLET = /^(?:[-*•]\s+|\d+[.)]\s+)?(?:\[([ xX])\]\s*)?/;

/**
 * Turns pasted multi-line text into items: one per non-blank line, with "-",
 * "*", "•" and "1." bullets removed, "[x]" marking the line done, and leading
 * indentation (two spaces or a tab per level) read as nesting.
 */
export function splitPastedList(text: string): PastedLine[] {
  const out: PastedLine[] = [];
  for (const raw of text.replace(/\r\n?/g, "\n").split("\n")) {
    if (raw.trim() === "") continue;
    const indent = raw.match(/^[ \t]*/)?.[0] ?? "";
    const depth = Math.floor(indent.replace(/\t/g, "  ").length / 2);
    const line = raw.trimStart();
    const m = line.match(BULLET);
    const rest = line.slice(m?.[0].length ?? 0).trim();
    if (rest === "") continue;
    out.push({ text: rest, done: m?.[1] === "x" || m?.[1] === "X", depth });
  }
  return out;
}

/** True when a paste should become multiple items rather than inline text. */
export function isMultiLinePaste(text: string): boolean {
  return splitPastedList(text).length > 1;
}

export type ItemInput = { kind: ItemKind; level: number; id: string; text: string; done: boolean; depth: number; body: string[] };

/**
 * What gets sent on save. Rows that are empty and have no notes are left out —
 * they are the blank line the user is about to type on, not an item — and
 * `kept` records which local rows the sent ones came from, so the reply can
 * be merged back by position.
 */
export function toInputs(items: LocalItem[]): { inputs: ItemInput[]; kept: number[] } {
  const inputs: ItemInput[] = [];
  const kept: number[] = [];
  items.forEach((it, i) => {
    if (it.text.trim() === "" && it.body.length === 0) return;
    inputs.push({ kind: it.kind, level: it.level, id: it.id, text: it.text, done: it.done, depth: it.depth, body: it.body });
    kept.push(i);
  });
  return { inputs, kept };
}

export type SavedMeta = {
  id: string;
  createdAt: string;
  doneAt: string;
  from: string;
  carried: number;
  recurring: string;
};

/**
 * Adopts the ids and stamps Go returned for the rows that were sent, without
 * disturbing anything the user typed since the save began. Text, done, depth
 * and body always come from the local row; only Go-owned metadata is taken
 * from the reply.
 */
export function mergeSaved(local: LocalItem[], saved: SavedMeta[], kept: number[]): LocalItem[] {
  const next = local.slice();
  saved.forEach((meta, i) => {
    const at = kept[i];
    if (at === undefined || !next[at]) return;
    next[at] = { ...next[at], ...meta };
  });
  return next;
}

export function fromServer(items: { kind: string; level: number; id: string; text: string; done: boolean; depth: number; body: string[] | null; createdAt: string; doneAt: string; from: string; carried: number; recurring: string }[]): LocalItem[] {
  return items.map((it) => ({
    kind: it.kind === "heading" ? "heading" : "",
    level: it.level,
    id: it.id,
    text: it.text,
    done: it.done,
    depth: it.depth,
    body: it.body ?? [],
    createdAt: it.createdAt,
    doneAt: it.doneAt,
    from: it.from,
    carried: it.carried,
    recurring: it.recurring,
    key: it.id || newKey(),
  }));
}

/** Typing "## " (any level) at the start of an empty row turns it into a heading. */
export function headingShortcut(text: string): { level: number; rest: string } | null {
  const m = text.match(/^(#{1,6})\s(.*)$/);
  return m ? { level: m[1].length, rest: m[2] } : null;
}
