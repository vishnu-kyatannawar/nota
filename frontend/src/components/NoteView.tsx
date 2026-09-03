import { useEffect, useMemo, useState } from "react";
import type { Note } from "../lib/api";
import { api, hhmm } from "../lib/api";
import { CodeEditor } from "./CodeEditor";
import { ItemRow } from "./ItemRow";

type Props = {
  note: Note;
  onChanged: () => void;
};

const DAY_TYPES = ["work", "weekend", "leave", "holiday"] as const;

export function NoteView({ note, onChanged }: Props) {
  const [raw, setRaw] = useState<string | null>(null);
  const [focusId, setFocusId] = useState<string | null>(null);
  const [draft, setDraft] = useState("");
  const [error, setError] = useState<string | null>(null);

  const isWorkplan = note.type === "workplan";
  const logged = useMemo(() => note.items.reduce((sum, i) => sum + i.minutes, 0), [note.items]);

  // Ctrl+E flips the whole note to its markdown source and back. Bound on the
  // document so it works wherever focus happens to be within the note.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "e" && (e.ctrlKey || e.metaKey)) {
        e.preventDefault();
        setRaw((current) => (current === null ? "" : null));
      }
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, []);

  useEffect(() => {
    if (raw !== "") return;
    api.raw(note.path).then(setRaw).catch((e) => setError(String(e)));
  }, [raw, note.path]);

  // Switching notes drops the raw-mode buffer and the half-typed new item, so
  // neither leaks from one note into the next.
  const [lastPath, setLastPath] = useState(note.path);
  if (note.path !== lastPath) {
    setLastPath(note.path);
    setRaw(null);
    setDraft("");
  }

  const run = async (fn: () => Promise<unknown>) => {
    try {
      setError(null);
      await fn();
      onChanged();
    } catch (e) {
      setError(String(e));
    }
  };

  const addItem = async (text: string, depth: number) => {
    if (!text.trim()) return;
    const id = await api.addItem(note.path, text, depth);
    setFocusId(id);
    onChanged();
  };

  const logTime = (id: string) => {
    const entry = window.prompt("Log time on this item (hh:mm)", "01:00");
    if (!entry) return;
    const match = entry.match(/^(\d{1,2}):([0-5]\d)$/);
    if (!match) {
      setError(`"${entry}" is not hh:mm — for example 01:20`);
      return;
    }
    void run(() =>
      api.addItemMinutes(note.path, id, Number(match[1]) * 60 + Number(match[2])),
    );
  };

  const setHours = () => {
    const suggestion = hhmm(logged);
    const entry = window.prompt(
      `Hours worked on ${note.date}. The item logs add up to ${suggestion}; this is only a suggestion.`,
      note.hours || suggestion,
    );
    if (!entry) return;
    void run(() => api.setHours(note.path, entry));
  };

  if (raw !== null) {
    return (
      <main className="flex min-w-0 flex-1 flex-col p-6">
        <Header title={note.title} subtitle="Markdown source · Ctrl+E to go back" />
        {error && <ErrorBar message={error} />}
        <div className="min-h-0 flex-1 overflow-auto">
          <CodeEditor
            value={raw}
            autoFocus
            minHeight="24rem"
            onChange={setRaw}
            onExit={() => setRaw(null)}
          />
        </div>
        <div className="mt-3 flex gap-2">
          <button
            type="button"
            onClick={() => void run(async () => { await api.saveRaw(note.path, raw); setRaw(null); })}
            className="rounded bg-accent px-3 py-1.5 text-sm font-medium text-black"
          >
            Save
          </button>
          <button
            type="button"
            onClick={() => setRaw(null)}
            className="rounded border border-surface-border px-3 py-1.5 text-sm"
          >
            Cancel
          </button>
        </div>
      </main>
    );
  }

  return (
    <main className="flex min-w-0 flex-1 flex-col overflow-y-auto p-6">
      <div className="mb-1 flex flex-wrap items-baseline gap-3">
        <h1 className="text-xl font-bold">{note.title}</h1>
        {isWorkplan && (
          <>
            <button
              type="button"
              onClick={setHours}
              className="rounded border border-surface-border px-2 py-0.5 font-mono text-xs text-amber-400 hover:border-accent/60"
            >
              {note.hours || "00:00"}
            </button>
            <select
              value={note.dayType || "work"}
              onChange={(e) => void run(() => api.setDayType(note.path, e.target.value))}
              aria-label="Day type"
              className="rounded border border-surface-border bg-surface px-1.5 py-0.5 text-xs text-ink-muted"
            >
              {DAY_TYPES.map((d) => (
                <option key={d} value={d}>
                  {d}
                </option>
              ))}
            </select>
          </>
        )}
        <span className="ml-auto font-mono text-xs text-ink-muted">
          {note.items.filter((i) => !i.done).length} open · {note.items.filter((i) => i.done).length} done
          {logged > 0 && ` · ${hhmm(logged)} on items`}
        </span>
      </div>
      <p className="mb-4 text-xs text-ink-muted/60">{note.path}</p>

      {error && <ErrorBar message={error} />}

      <div className="space-y-0.5">
        {note.items.map((item) => (
          <ItemRow
            key={item.id || item.text}
            item={item}
            focusOnMount={item.id === focusId}
            onToggle={() => void run(() => api.setItemDone(note.path, item.id, !item.done))}
            onText={(text) => void run(() => api.setItemText(note.path, item.id, text))}
            onEnter={() => void addItem("", item.depth)}
            onIndent={(delta) => {
              const depth = Math.max(0, item.depth + delta);
              if (depth === item.depth) return;
              void run(async () => {
                await api.removeItem(note.path, item.id);
                await api.addItem(note.path, item.text, depth);
              });
            }}
            onRemove={() => void run(() => api.removeItem(note.path, item.id))}
            onLogTime={() => logTime(item.id)}
            onBody={(body) => void run(() => api.setItemBody(note.path, item.id, body))}
          />
        ))}
      </div>

      <form
        onSubmit={(e) => {
          e.preventDefault();
          void addItem(draft, 0);
          setDraft("");
        }}
        className="mt-1 flex items-center gap-2 px-1"
      >
        <span className="h-[15px] w-[15px] shrink-0 rounded-sm border border-dashed border-white/20" />
        <input
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          placeholder="Type an item and press Enter"
          aria-label="New item"
          className="min-w-0 flex-1 bg-transparent py-1 text-sm outline-none placeholder:text-ink-muted/40"
        />
      </form>

      {note.body && (
        <div className="mt-6 whitespace-pre-wrap border-t border-surface-border pt-4 text-sm text-ink-muted">
          {note.body}
        </div>
      )}

      <div className="mt-6 flex flex-wrap gap-3 border-t border-surface-border pt-3 text-[11px] text-ink-muted/50">
        <Key label="Enter">new item</Key>
        <Key label="Tab">indent</Key>
        <Key label="Ctrl+Enter">toggle done</Key>
        <Key label="Ctrl+T">log time</Key>
        <Key label="Ctrl+Shift+N">item notes</Key>
        <Key label="Ctrl+E">raw markdown</Key>
      </div>
    </main>
  );
}

function Header({ title, subtitle }: { title: string; subtitle: string }) {
  return (
    <div className="mb-3">
      <h1 className="text-xl font-bold">{title}</h1>
      <p className="text-xs text-ink-muted/60">{subtitle}</p>
    </div>
  );
}

function ErrorBar({ message }: { message: string }) {
  return (
    <p role="alert" className="mb-3 rounded border border-red-900 bg-red-950/40 p-2 text-xs text-red-300">
      {message}
    </p>
  );
}

function Key({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <span>
      <kbd className="rounded border border-surface-border px-1 font-mono">{label}</kbd> {children}
    </span>
  );
}
