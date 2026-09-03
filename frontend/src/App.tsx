import { useCallback, useEffect, useState } from "react";
import { Events } from "@wailsio/runtime";
import type { Hit, Label, Node, Note, Workplan } from "./lib/api";
import { api } from "./lib/api";
import { Sidebar } from "./components/Sidebar";
import { NoteView } from "./components/NoteView";

export default function App() {
  const [tree, setTree] = useState<Node | null>(null);
  const [workplans, setWorkplans] = useState<Workplan[]>([]);
  const [labels, setLabels] = useState<Label[]>([]);
  const [weekHours, setWeekHours] = useState("00:00");
  const [note, setNote] = useState<Note | null>(null);
  const [hits, setHits] = useState<Hit[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  const refreshShell = useCallback(async () => {
    const [t, w, l, h] = await Promise.all([
      api.tree(),
      api.workplans(),
      api.labels(),
      api.hoursThisWeek(),
    ]);
    setTree(t);
    setWorkplans(w);
    setLabels(l);
    setWeekHours(h.hours);
  }, []);

  const open = useCallback(async (path: string) => {
    try {
      setHits(null);
      setNote(await api.note(path));
    } catch (e) {
      setError(String(e));
    }
  }, []);

  const refresh = useCallback(async () => {
    try {
      await refreshShell();
      if (note) setNote(await api.note(note.path));
    } catch (e) {
      setError(String(e));
    }
  }, [note, refreshShell]);

  // On launch, make sure today's workplan exists and open it. Ensure is
  // idempotent, so this is safe on every start of the day or later.
  useEffect(() => {
    (async () => {
      try {
        const today = await api.ensureToday();
        await refreshShell();
        if (today) await open(today);
      } catch (e) {
        setError(String(e));
      }
    })();
    // Deliberately runs once, on mount.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // A note edited outside the application, or the midnight rollover, both reach
  // the window as events so it never shows stale content.
  useEffect(() => {
    Events.On("note:changed", () => void refresh());
    Events.On("workplan:rolled", (ev: { data: string }) => {
      void refreshShell();
      if (ev?.data) void open(ev.data);
    });
    return () => {
      Events.Off("note:changed");
      Events.Off("workplan:rolled");
    };
  }, [refresh, refreshShell, open]);

  const search = useCallback(async (query: string) => {
    if (!query.trim()) {
      setHits(null);
      return;
    }
    try {
      setHits(await api.search(query));
    } catch (e) {
      setError(String(e));
    }
  }, []);

  const byLabel = useCallback(async (name: string) => {
    try {
      const paths = await api.notesByLabel(name);
      setHits(paths.map((path) => ({ path, snippet: `#${name}` })));
    } catch (e) {
      setError(String(e));
    }
  }, []);

  return (
    <div className="flex h-full">
      <Sidebar
        tree={tree}
        workplans={workplans}
        labels={labels}
        weekHours={weekHours}
        current={note?.path ?? ""}
        onOpen={open}
        onSearch={(q) => void search(q)}
        onLabel={(n) => void byLabel(n)}
      />

      {hits !== null ? (
        <main className="min-w-0 flex-1 overflow-y-auto p-6">
          <h1 className="mb-4 text-xl font-bold">
            {hits.length} {hits.length === 1 ? "result" : "results"}
          </h1>
          <div className="space-y-1">
            {hits.map((hit) => (
              <button
                key={hit.path}
                type="button"
                onClick={() => void open(hit.path)}
                className="block w-full rounded px-3 py-2 text-left hover:bg-white/5"
              >
                <div className="font-mono text-sm text-accent">{hit.path}</div>
                {hit.snippet && <div className="text-xs text-ink-muted">{hit.snippet}</div>}
              </button>
            ))}
            {hits.length === 0 && <p className="text-sm text-ink-muted">Nothing matched.</p>}
          </div>
        </main>
      ) : note ? (
        <NoteView note={note} onChanged={() => void refresh()} />
      ) : (
        <main className="flex min-w-0 flex-1 items-center justify-center p-6">
          <p className="text-sm text-ink-muted">
            {error ?? "Select a note, or wait for today's workplan to open."}
          </p>
        </main>
      )}

      {error && (
        <div
          role="alert"
          className="fixed bottom-4 right-4 max-w-md rounded border border-red-900 bg-red-950/90 p-3 text-xs text-red-200"
        >
          {error}
          <button type="button" onClick={() => setError(null)} className="ml-3 underline">
            dismiss
          </button>
        </div>
      )}
    </div>
  );
}
