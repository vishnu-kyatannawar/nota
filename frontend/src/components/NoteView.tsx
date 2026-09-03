import { useCallback, useEffect, useReducer, useRef, useState } from "react";
import type { Note } from "../lib/api";
import { api, HHMM } from "../lib/api";
import { blankItem, fromServer, initialState, mergeSaved, reducer, toInputs } from "../lib/items";
import { CodeEditor } from "./CodeEditor";
import { ItemRow } from "./ItemRow";

type Props = {
  path: string;
  dark: boolean;
  /** Bumped by the parent when the file changed on disk. */
  reloadToken: number;
  onShellChanged: () => void;
  onError: (message: string) => void;
};

const DAY_TYPES = ["work", "weekend", "leave", "holiday"] as const;
const SAVE_DELAY = 400;

export function NoteView({ path, dark, reloadToken, onShellChanged, onError }: Props) {
  const [note, setNote] = useState<Note | null>(null);
  const [state, dispatch] = useReducer(reducer, initialState);
  const [raw, setRaw] = useState<string | null>(null);
  const [saving, setSaving] = useState<"idle" | "saving" | "saved">("idle");
  const [editingHours, setEditingHours] = useState(false);
  const [hoursDraft, setHoursDraft] = useState("");

  // Refs let the save and reload logic see the latest state without being
  // re-created on every keystroke.
  const stateRef = useRef(state);
  useEffect(() => {
    stateRef.current = state;
  });
  const pathRef = useRef(path);
  const inFlight = useRef(false);
  const reloadPending = useRef(false);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const load = useCallback(
    (p: string) =>
      api
        .note(p)
        .then((n) => {
          if (pathRef.current !== p) return;
          setNote(n);
          const items = fromServer(n.items);
          // An empty note shows one blank row to type on; it is not an item
          // until it has text, so this does not mark the note dirty.
          dispatch({ type: "replaceAll", items: items.length ? items : [blankItem()] });
          if (items.length === 0) dispatch({ type: "focus", index: 0 });
        })
        .catch((e) => onError(String(e))),
    [onError],
  );

  const save = useCallback(async (): Promise<void> => {
    const s = stateRef.current;
    const p = pathRef.current;
    if (!s.dirty || inFlight.current) return;
    inFlight.current = true;
    setSaving("saving");
    const { inputs, kept } = toInputs(s.items);
    try {
      const saved = await api.saveItems(p, inputs);
      if (pathRef.current !== p) return;
      const latest = stateRef.current;
      const merged = mergeSaved(
        latest.items,
        saved.map((it) => ({ id: it.ID, createdAt: it.CreatedAt, doneAt: it.DoneAt, from: it.From, carried: it.Carried, recurring: it.Recurring })),
        kept,
      );
      // If the user kept typing during the round trip the note is still dirty
      // and another save will follow; otherwise it is clean now.
      dispatch({ type: "replaceAll", items: merged, dirty: latest.items !== s.items });
      setSaving("saved");
      onShellChanged();
    } catch (e) {
      onError(String(e));
      setSaving("idle");
    } finally {
      inFlight.current = false;
      if (reloadPending.current && !stateRef.current.dirty) {
        reloadPending.current = false;
        void load(pathRef.current);
      }
    }
  }, [load, onError, onShellChanged]);

  // Switching notes flushes whatever is unsaved on the old one, then loads.
  useEffect(() => {
    const previous = pathRef.current;
    if (previous !== path && stateRef.current.dirty) {
      const s = stateRef.current;
      const { inputs } = toInputs(s.items);
      void api.saveItems(previous, inputs).then(onShellChanged).catch((e) => onError(String(e)));
    }
    pathRef.current = path;
    void load(path);
  }, [path, load, onError, onShellChanged]);

  // Switching notes drops the raw-mode buffer and any half-edited hours field,
  // so neither leaks from one note into the next. Adjusting state during render
  // is React's own pattern for this.
  const [lastPath, setLastPath] = useState(path);
  if (path !== lastPath) {
    setLastPath(path);
    setRaw(null);
    setEditingHours(false);
  }

  // Debounced save while typing.
  useEffect(() => {
    if (!state.dirty) return;
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(() => void save(), SAVE_DELAY);
    return () => {
      if (timer.current) clearTimeout(timer.current);
    };
  }, [state, save]);

  // A change on disk is adopted only when nothing local is unsaved, so an
  // external edit can never clobber a word mid-typing.
  useEffect(() => {
    if (reloadToken === 0) return;
    if (stateRef.current.dirty || inFlight.current) {
      reloadPending.current = true;
      return;
    }
    void load(pathRef.current);
  }, [reloadToken, load]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "e" && (e.ctrlKey || e.metaKey)) {
        e.preventDefault();
        setRaw((r) => (r === null ? "" : null));
      }
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, []);

  useEffect(() => {
    if (raw !== "") return;
    api.raw(path).then(setRaw).catch((e) => onError(String(e)));
  }, [raw, path, onError]);

  if (!note) return <main className="flex-1" />;

  const isWorkplan = note.type === "workplan";
  const open = state.items.filter((i) => !i.done && i.text.trim() !== "").length;
  const done = state.items.filter((i) => i.done).length;

  const commitHours = async () => {
    const value = hoursDraft.trim();
    setEditingHours(false);
    if (!HHMM.test(value)) {
      onError(`"${value}" is not hh:mm — for example 07:30`);
      return;
    }
    try {
      await api.setHours(path, value);
      setNote({ ...note, hours: value, title: `${note.date} - ${value}` });
      onShellChanged();
    } catch (e) {
      onError(String(e));
    }
  };

  const setDayType = async (dayType: string) => {
    try {
      await api.setDayType(path, dayType);
      setNote({ ...note, dayType });
      onShellChanged();
    } catch (e) {
      onError(String(e));
    }
  };

  if (raw !== null) {
    return (
      <main className="flex min-w-0 flex-1 flex-col overflow-hidden">
        <Header note={note} subtitle="Markdown source · Ctrl+E to return" />
        <div className="min-h-0 flex-1 overflow-auto px-8 pb-4">
          <CodeEditor value={raw} autoFocus minHeight="24rem" dark={dark} onChange={setRaw} onExit={() => setRaw(null)} />
        </div>
        <div className="flex gap-2 border-t border-border px-8 py-3">
          <button
            type="button"
            onClick={() => void api.saveRaw(path, raw).then(() => { setRaw(null); void load(path); onShellChanged(); }).catch((e) => onError(String(e)))}
            className="rounded-md bg-accent px-3 py-1.5 text-sm font-medium text-accent-ink hover:opacity-90"
          >
            Save
          </button>
          <button type="button" onClick={() => setRaw(null)} className="rounded-md border border-border px-3 py-1.5 text-sm hover:bg-surface-raised">
            Cancel
          </button>
        </div>
      </main>
    );
  }

  return (
    <main className="flex min-w-0 flex-1 flex-col overflow-hidden">
      <div className="flex flex-wrap items-center gap-3 border-b border-border px-8 py-3">
        <h1 className="text-[15px] font-semibold tracking-tight">
          {isWorkplan ? note.date : note.title}
        </h1>
        {isWorkplan && (
          <>
            <span className="text-ink-faint">–</span>
            {editingHours ? (
              <input
                autoFocus
                value={hoursDraft}
                onChange={(e) => setHoursDraft(e.target.value)}
                onBlur={() => void commitHours()}
                onKeyDown={(e) => {
                  if (e.key === "Enter") void commitHours();
                  if (e.key === "Escape") setEditingHours(false);
                }}
                placeholder="hh:mm"
                aria-label="Hours worked today"
                className="w-16 rounded-md border border-accent bg-surface-raised px-2 py-0.5 font-mono text-sm outline-none"
              />
            ) : (
              <button
                type="button"
                onClick={() => { setHoursDraft(note.hours || "00:00"); setEditingHours(true); }}
                title="Hours worked today — click to edit"
                className="rounded-md border border-border bg-surface-raised px-2 py-0.5 font-mono text-sm text-ink hover:border-accent"
              >
                {note.hours || "00:00"}
              </button>
            )}
            <select
              value={note.dayType || "work"}
              onChange={(e) => void setDayType(e.target.value)}
              aria-label="Day type"
              className="rounded-md border border-border bg-surface-raised px-1.5 py-0.5 text-xs text-ink-muted"
            >
              {DAY_TYPES.map((d) => <option key={d} value={d}>{d}</option>)}
            </select>
          </>
        )}
        <span className="ml-auto flex items-center gap-3 text-xs text-ink-muted">
          <span>{open} open · {done} done</span>
          <span className={`transition ${saving === "saving" ? "text-ink-faint" : saving === "saved" ? "text-success" : "opacity-0"}`}>
            {saving === "saving" ? "Saving…" : "Saved"}
          </span>
        </span>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto px-6 py-4">
        <p className="mb-3 px-1.5 font-mono text-[11px] text-ink-faint">{note.path}</p>

        <div className="space-y-px">
          {state.items.map((item, index) => (
            <ItemRow
              key={item.key}
              item={item}
              index={index}
              focused={state.focus === index}
              caret={state.caret}
              dispatch={dispatch}
              dark={dark}
            />
          ))}
        </div>

        <button
          type="button"
          onClick={() => dispatch({ type: "insertAfter", index: state.items.length - 1 })}
          className="mt-1 flex items-center gap-2 rounded-md px-1.5 py-1 text-sm text-ink-faint hover:bg-surface-raised hover:text-ink-muted"
        >
          <span className="flex h-4 w-4 items-center justify-center rounded border border-dashed border-border-strong text-[11px]">+</span>
          Add item
        </button>

        {note.body && (
          <div className="mt-8 whitespace-pre-wrap border-t border-border pt-4 text-sm text-ink-muted">{note.body}</div>
        )}
      </div>

      <div className="flex flex-wrap gap-x-4 gap-y-1 border-t border-border px-8 py-2 text-[11px] text-ink-faint">
        <Key k="Enter">new item</Key>
        <Key k="↑ ↓">move</Key>
        <Key k="Tab">indent</Key>
        <Key k="Ctrl+Enter">done</Key>
        <Key k="Ctrl+Shift+N">notes</Key>
        <Key k="Ctrl+E">markdown</Key>
        <span className="ml-auto">paste a list to add many at once</span>
      </div>
    </main>
  );
}

function Header({ note, subtitle }: { note: Note; subtitle: string }) {
  return (
    <div className="border-b border-border px-8 py-3">
      <h1 className="text-[15px] font-semibold">{note.title}</h1>
      <p className="text-xs text-ink-faint">{subtitle}</p>
    </div>
  );
}

function Key({ k, children }: { k: string; children: React.ReactNode }) {
  return (
    <span>
      <kbd className="rounded border border-border bg-surface-raised px-1 font-mono text-[10px]">{k}</kbd> {children}
    </span>
  );
}
