import { useCallback, useEffect, useReducer, useRef, useState } from "react";
import type { Note } from "../lib/api";
import { api, HHMM } from "../lib/api";
import { blankItem, fromServer, initialState, keepUnsaved, mergeSaved, reducer, toInputs } from "../lib/items";
import { cleanLabel } from "../lib/labels";
import { stealsFocus } from "../lib/focus";
import { CodeEditor } from "./CodeEditor";
import { ItemRow } from "./ItemRow";
import { NotesEditor } from "./NotesEditor";

type Props = {
  path: string;
  dark: boolean;
  /** Bumped by the parent when the file changed on disk. */
  reloadToken: number;
  allLabels: string[];
  todayPath: string | null;
  onShellChanged: () => void;
  onError: (message: string) => void;
};

const DAY_TYPES = ["work", "weekend", "leave", "holiday"] as const;
const LAYOUTS = ["items", "notes", "both"] as const;
const ITEM_SAVE_DELAY = 400;
const BODY_SAVE_DELAY = 600;

export function NoteView({ path, dark, reloadToken, allLabels, todayPath, onShellChanged, onError }: Props) {
  const [note, setNote] = useState<Note | null>(null);
  const [state, dispatch] = useReducer(reducer, initialState);
  const [body, setBody] = useState("");
  const [raw, setRaw] = useState<string | null>(null);
  const [saving, setSaving] = useState<"idle" | "saving" | "saved">("idle");
  const [editingHours, setEditingHours] = useState(false);
  const [hoursDraft, setHoursDraft] = useState("");
  const [labelDraft, setLabelDraft] = useState("");

  // Refs let the save and reload logic read the latest state without being
  // re-created on every keystroke.
  const stateRef = useRef(state);
  useEffect(() => {
    stateRef.current = state;
  });
  const bodyRef = useRef({ value: "", dirty: false });
  const pathRef = useRef(path);
  // Which path the current items belong to, so a reload of the same note can
  // keep its unsaved rows while a switch to another note cannot.
  const loadedPath = useRef("");
  const inFlight = useRef(false);
  const reloadPending = useRef(false);
  const itemTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const bodyTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const anyDirty = () => stateRef.current.dirty || bodyRef.current.dirty || inFlight.current;

  const load = useCallback(
    (p: string) =>
      api
        .note(p)
        .then((n) => {
          if (pathRef.current !== p) return;
          setNote(n);
          const showsItems = n.type === "workplan" || n.layout !== "notes";
          // Reloading this same note must not take away a row the user just
          // made and has not typed into yet — saving is what triggers the
          // reload, so that row would vanish under them.
          const items = loadedPath.current === p
            ? keepUnsaved(stateRef.current.items, fromServer(n.items))
            : fromServer(n.items);
          loadedPath.current = p;
          // An empty items section shows one blank row to type on; it is not
          // an item until it has text, so this does not mark the note dirty.
          dispatch({ type: "replaceAll", items: items.length || !showsItems ? items : [blankItem()] });
          // A load can land while the user is renaming this note in the sidebar;
          // taking the caret then is what made rename look broken.
          if (items.length === 0 && showsItems && !stealsFocus()) dispatch({ type: "focus", index: 0 });
          setBody(n.body);
          bodyRef.current = { value: n.body, dirty: false };
        })
        .catch((e) => onError(String(e))),
    [onError],
  );

  const afterSave = useCallback(() => {
    if (reloadPending.current && !anyDirty()) {
      reloadPending.current = false;
      void load(pathRef.current);
    }
  }, [load]);

  const saveItems = useCallback(async (): Promise<void> => {
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
      dispatch({ type: "replaceAll", items: merged, dirty: latest.items !== s.items });
      setSaving("saved");
      onShellChanged();
    } catch (e) {
      onError(String(e));
      setSaving("idle");
    } finally {
      inFlight.current = false;
      afterSave();
    }
  }, [afterSave, onError, onShellChanged]);

  const saveBody = useCallback(async (): Promise<void> => {
    const b = bodyRef.current;
    const p = pathRef.current;
    if (!b.dirty) return;
    bodyRef.current = { ...b, dirty: false };
    setSaving("saving");
    try {
      await api.saveBody(p, b.value);
      if (pathRef.current !== p) return;
      setSaving("saved");
      onShellChanged();
    } catch (e) {
      bodyRef.current.dirty = true;
      onError(String(e));
      setSaving("idle");
    } finally {
      afterSave();
    }
  }, [afterSave, onError, onShellChanged]);

  // Switching notes flushes whatever is unsaved on the old one, then loads.
  useEffect(() => {
    const previous = pathRef.current;
    if (previous !== path) {
      if (stateRef.current.dirty) {
        const { inputs } = toInputs(stateRef.current.items);
        void api.saveItems(previous, inputs).then(onShellChanged).catch((e) => onError(String(e)));
      }
      if (bodyRef.current.dirty) {
        void api.saveBody(previous, bodyRef.current.value).then(onShellChanged).catch((e) => onError(String(e)));
        bodyRef.current.dirty = false;
      }
    }
    pathRef.current = path;
    void load(path);
  }, [path, load, onError, onShellChanged]);

  const [lastPath, setLastPath] = useState(path);
  if (path !== lastPath) {
    setLastPath(path);
    setRaw(null);
    setEditingHours(false);
    setLabelDraft("");
  }

  useEffect(() => {
    if (!state.dirty) return;
    if (itemTimer.current) clearTimeout(itemTimer.current);
    itemTimer.current = setTimeout(() => void saveItems(), ITEM_SAVE_DELAY);
    return () => {
      if (itemTimer.current) clearTimeout(itemTimer.current);
    };
  }, [state, saveItems]);

  const onBodyChange = (markdown: string) => {
    setBody(markdown);
    bodyRef.current = { value: markdown, dirty: true };
    if (bodyTimer.current) clearTimeout(bodyTimer.current);
    bodyTimer.current = setTimeout(() => void saveBody(), BODY_SAVE_DELAY);
  };

  // A change on disk is adopted only when nothing local is unsaved.
  useEffect(() => {
    if (reloadToken === 0) return;
    if (anyDirty()) {
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
  const layout = isWorkplan ? "both" : note.layout;
  const showItems = layout !== "notes";
  const showNotes = layout !== "items";
  const isToday = todayPath === path;
  const open = state.items.filter((i) => i.kind !== "heading" && !i.done && i.text.trim() !== "").length;
  const done = state.items.filter((i) => i.kind !== "heading" && i.done).length;

  const run = async (fn: () => Promise<unknown>, then?: () => void) => {
    try {
      await fn();
      then?.();
      onShellChanged();
    } catch (e) {
      onError(String(e));
    }
  };

  const commitHours = async () => {
    const value = hoursDraft.trim();
    setEditingHours(false);
    if (!HHMM.test(value)) {
      onError(`"${value}" is not hh:mm — for example 07:30`);
      return;
    }
    await run(() => api.setHours(path, value), () => setNote({ ...note, hours: value, title: `${note.date} - ${value}` }));
  };

  const setLayout = (next: (typeof LAYOUTS)[number]) =>
    run(() => api.setLayout(path, next), () => {
      setNote({ ...note, layout: next });
      if (next !== "notes" && state.items.length === 0) {
        dispatch({ type: "replaceAll", items: [blankItem()] });
        if (!stealsFocus()) dispatch({ type: "focus", index: 0 });
      }
    });

  const setLabels = (labels: string[]) => run(() => api.setLabels(path, labels), () => setNote({ ...note, labels }));

  const moveToToday = async (indexes: number[]) => {
    await saveItems(); // new rows need their ids minted first
    const ids = indexes.map((i) => stateRef.current.items[i]?.id).filter((id): id is string => Boolean(id));
    if (ids.length === 0) return;
    await run(() => api.moveItems(path, ids, ""), () => void load(path));
  };

  if (raw !== null) {
    return (
      <main className="flex min-w-0 flex-1 flex-col overflow-hidden">
        <div className="border-b border-border px-8 py-3">
          <h1 className="text-[15px] font-semibold">{note.title}</h1>
          <p className="text-xs text-ink-faint">Markdown source · Ctrl+E to return</p>
        </div>
        <div className="min-h-0 flex-1 overflow-auto px-8 pb-4">
          <CodeEditor value={raw} autoFocus minHeight="24rem" dark={dark} onChange={setRaw} onExit={() => setRaw(null)} />
        </div>
        <div className="flex gap-2 border-t border-border px-8 py-3">
          <button
            type="button"
            onClick={() => void run(() => api.saveRaw(path, raw), () => { setRaw(null); void load(path); })}
            className="rounded-md bg-accent px-3 py-1.5 text-sm font-medium text-accent-ink hover:opacity-90"
          >
            Save
          </button>
          <button type="button" onClick={() => setRaw(null)} className="rounded-md border border-border px-3 py-1.5 text-sm hover:bg-surface-raised">Cancel</button>
        </div>
      </main>
    );
  }

  return (
    <main className="flex min-w-0 flex-1 flex-col overflow-hidden">
      <div className="flex flex-wrap items-center gap-3 border-b border-border px-8 py-3">
        <h1 className="text-[15px] font-semibold tracking-tight">{isWorkplan ? note.date : note.title}</h1>

        {isWorkplan ? (
          <>
            <span className="text-ink-faint">–</span>
            {editingHours ? (
              <input
                autoFocus
                value={hoursDraft}
                onChange={(e) => setHoursDraft(e.target.value)}
                onBlur={() => void commitHours()}
                onKeyDown={(e) => { if (e.key === "Enter") void commitHours(); if (e.key === "Escape") setEditingHours(false); }}
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
              onChange={(e) => void run(() => api.setDayType(path, e.target.value), () => setNote({ ...note, dayType: e.target.value }))}
              aria-label="Day type"
              className="rounded-md border border-border bg-surface-raised px-1.5 py-0.5 text-xs text-ink-muted"
            >
              {DAY_TYPES.map((d) => <option key={d} value={d}>{d}</option>)}
            </select>
          </>
        ) : (
          <div role="radiogroup" aria-label="Layout" className="flex rounded-md border border-border p-0.5 text-[12px]">
            {LAYOUTS.map((l) => (
              <button
                key={l}
                type="button"
                role="radio"
                aria-checked={layout === l}
                onClick={() => void setLayout(l)}
                className={`rounded px-2 py-0.5 capitalize ${layout === l ? "bg-accent-soft text-accent" : "text-ink-muted hover:text-ink"}`}
              >
                {l}
              </button>
            ))}
          </div>
        )}

        <span className="ml-auto flex items-center gap-3 text-xs text-ink-muted">
          {showItems && <span>{open} open · {done} done</span>}
          <span className={`transition ${saving === "saving" ? "text-ink-faint" : saving === "saved" ? "text-success" : "opacity-0"}`}>
            {saving === "saving" ? "Saving…" : "Saved"}
          </span>
        </span>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto px-6 py-4">
        <div className="mb-3 flex flex-wrap items-center gap-1.5 px-1.5">
          <span className="font-mono text-[11px] text-ink-muted">{note.path}</span>
          <span className="mx-1 text-ink-faint">·</span>
          {note.labels.map((l) => (
            <span key={l} className="inline-flex items-center gap-0.5 rounded-full bg-accent-soft pl-1.5 pr-0.5 text-[11px] leading-5 text-accent">
              #{l}
              <button type="button" aria-label={`Remove label ${l}`} onClick={() => void setLabels(note.labels.filter((x) => x !== l))} className="rounded-full px-1 hover:bg-accent hover:text-accent-ink">×</button>
            </span>
          ))}
          <input
            value={labelDraft}
            onChange={(e) => setLabelDraft(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                const name = cleanLabel(labelDraft);
                if (name && !note.labels.includes(name)) void setLabels([...note.labels, name]);
                setLabelDraft("");
              }
              if (e.key === "Escape") setLabelDraft("");
            }}
            placeholder="+ label"
            aria-label="Add a label to this page"
            className="w-20 bg-transparent text-[11px] text-ink-muted outline-none placeholder:text-ink-faint focus:w-32"
          />
        </div>

        {showItems && (
          <section>
            {!isWorkplan && (
              <SectionCaption>
                Items
                {layout === "items" && !isToday && open > 0 && (
                  <button
                    type="button"
                    onClick={() => void moveToToday(state.items.map((it, i) => (it.kind !== "heading" && !it.done && it.text.trim() ? i : -1)).filter((i) => i >= 0))}
                    className="ml-auto rounded px-1.5 py-0.5 text-[11px] normal-case tracking-normal text-accent hover:bg-accent-soft"
                  >
                    Move open items to today →
                  </button>
                )}
              </SectionCaption>
            )}
            <div className="space-y-px">
              {state.items.map((item, index) => (
                <ItemRow
                  key={item.key}
                  item={item}
                  index={index}
                  focused={state.focus === index}
                  caret={state.caret}
                  dispatch={dispatch}
                  onError={onError}
                  dark={dark}
                  allLabels={allLabels}
                  onMoveToToday={isToday ? undefined : () => void moveToToday([index])}
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
          </section>
        )}

        {!showItems && note.items.length > 0 && (
          <Hint onClick={() => void setLayout("both")}>{note.items.length} {note.items.length === 1 ? "item" : "items"} hidden — show</Hint>
        )}

        {showNotes && (
          <section className={showItems ? "mt-6" : ""}>
            {(isWorkplan || showItems) && <SectionCaption>Notes</SectionCaption>}
            <NotesEditor value={body} onChange={onBodyChange} onBlur={() => void saveBody()} onError={onError} placeholder={isWorkplan ? "Notes for the day…" : "Write…"} />
          </section>
        )}

        {!showNotes && note.body.trim() !== "" && (
          <Hint onClick={() => void setLayout("both")}>Notes hidden — show</Hint>
        )}
      </div>

      <div className="flex flex-wrap gap-x-4 gap-y-1 border-t border-border px-8 py-2 text-[12px] text-ink-muted">
        <Key k="Enter">new item</Key>
        <Key k="↑ ↓">move</Key>
        <Key k="Tab">indent</Key>
        <Key k="Ctrl+Enter">done</Key>
        <Key k="Ctrl+⌫">delete</Key>
        <Key k="## ">heading</Key>
        <Key k="#">label</Key>
        <Key k="Ctrl+Shift+N">notes</Key>
        {!isToday && <Key k="Ctrl+Shift+M">to today</Key>}
        <Key k="Ctrl+E">markdown</Key>
      </div>
    </main>
  );
}

function SectionCaption({ children }: { children: React.ReactNode }) {
  return (
    <div className="mb-1.5 flex items-center border-b border-border px-1.5 pb-1 text-[11px] font-semibold uppercase tracking-[0.1em] text-ink-muted">
      {children}
    </div>
  );
}

function Hint({ onClick, children }: { onClick: () => void; children: React.ReactNode }) {
  return (
    <button type="button" onClick={onClick} className="mt-3 rounded-md border border-dashed border-border px-3 py-1.5 text-[12px] text-ink-muted hover:border-accent hover:text-accent">
      {children}
    </button>
  );
}

function Key({ k, children }: { k: string; children: React.ReactNode }) {
  return (
    <span>
      <kbd className="rounded border border-border-strong bg-surface-sunken px-1 font-mono text-[11px] text-ink">{k}</kbd> {children}
    </span>
  );
}
