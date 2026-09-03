import { useCallback, useEffect, useState } from "react";
import { Events } from "@wailsio/runtime";
import type { Hit, Info, Label, Node, Workplan } from "./lib/api";
import { api } from "./lib/api";
import { applyTheme, isTheme, resolvedTheme, THEMES, type Theme } from "./lib/theme";
import { Sidebar } from "./components/Sidebar";
import { NoteView } from "./components/NoteView";
import { AboutDialog } from "./components/AboutDialog";
import { SettingsDialog } from "./components/SettingsDialog";
import { ConfirmDialog } from "./components/ConfirmDialog";

export default function App() {
  const [info, setInfo] = useState<Info | null>(null);
  const [theme, setTheme] = useState<Theme>("system");
  const [dark, setDark] = useState(false);
  const [tree, setTree] = useState<Node | null>(null);
  const [workplans, setWorkplans] = useState<Workplan[]>([]);
  const [labels, setLabels] = useState<Label[]>([]);
  const [weekHours, setWeekHours] = useState("00:00");
  const [current, setCurrent] = useState<string | null>(null);
  const [reloadToken, setReloadToken] = useState(0);
  const [hits, setHits] = useState<Hit[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [renaming, setRenaming] = useState<string | null>(null);
  const [about, setAbout] = useState(false);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [confirm, setConfirm] = useState<{ node: Node } | null>(null);

  const fail = useCallback((m: string) => setError(m), []);

  const refreshShell = useCallback(async () => {
    try {
      const [t, w, l, h] = await Promise.all([api.tree(), api.workplans(), api.labels(), api.hoursThisWeek()]);
      setTree(t);
      setWorkplans(w);
      setLabels(l);
      setWeekHours(h.hours);
    } catch (e) {
      fail(String(e));
    }
  }, [fail]);

  const open = useCallback((path: string) => {
    setHits(null);
    setCurrent(path);
  }, []);

  // Theme: applied to <html>; "system" tracks the OS live.
  const chooseTheme = useCallback((t: Theme) => {
    setTheme(t);
    applyTheme(t);
    setDark(resolvedTheme(t) === "dark");
    api.setTheme(t).catch((e) => fail(String(e)));
  }, [fail]);

  useEffect(() => {
    const mq = window.matchMedia("(prefers-color-scheme: dark)");
    const onChange = () => setDark(resolvedTheme(theme) === "dark");
    mq.addEventListener("change", onChange);
    return () => mq.removeEventListener("change", onChange);
  }, [theme]);

  useEffect(() => {
    (async () => {
      try {
        const i = await api.info();
        setInfo(i);
        const t = isTheme(i.theme) ? i.theme : "system";
        setTheme(t);
        applyTheme(t);
        setDark(resolvedTheme(t) === "dark");
        const today = await api.ensureToday();
        await refreshShell();
        if (today) setCurrent(today);
      } catch (e) {
        fail(String(e));
      }
    })();
    // Runs once on mount.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    Events.On("note:changed", () => {
      setReloadToken((n) => n + 1);
      void refreshShell();
    });
    Events.On("workplan:rolled", (ev: { data: string }) => {
      void refreshShell();
      if (ev?.data) open(ev.data);
    });
    return () => {
      Events.Off("note:changed");
      Events.Off("workplan:rolled");
    };
  }, [refreshShell, open]);

  const search = useCallback(async (q: string) => {
    if (!q.trim()) {
      setHits(null);
      return;
    }
    try {
      setHits(await api.search(q));
    } catch (e) {
      fail(String(e));
    }
  }, [fail]);

  const byLabel = useCallback(async (name: string) => {
    try {
      const paths = await api.notesByLabel(name);
      setHits(paths.map((path) => ({ path, snippet: `#${name}` })));
    } catch (e) {
      fail(String(e));
    }
  }, [fail]);

  const newNote = useCallback(async (folder: string) => {
    try {
      const path = await api.createNote(`${folder ? folder + "/" : ""}Untitled.md`);
      await refreshShell();
      open(path);
      setRenaming(path);
    } catch (e) {
      fail(String(e));
    }
  }, [fail, open, refreshShell]);

  const newFolder = useCallback(async (folder: string) => {
    try {
      let name = "New folder";
      const siblings = new Set((folder ? findNode(tree, folder)?.children : tree?.children)?.map((c) => c.name) ?? []);
      for (let i = 2; siblings.has(name); i++) name = `New folder ${i}`;
      const path = `${folder ? folder + "/" : ""}${name}`;
      await api.createFolder(path);
      await refreshShell();
      setRenaming(path);
    } catch (e) {
      fail(String(e));
    }
  }, [fail, refreshShell, tree]);

  const rename = useCallback(async (from: string, newName: string) => {
    const parent = from.includes("/") ? from.slice(0, from.lastIndexOf("/") + 1) : "";
    const isNote = from.endsWith(".md");
    const to = `${parent}${newName.replace(/[/\\]/g, "-")}${isNote && !newName.endsWith(".md") ? ".md" : ""}`;
    try {
      await api.rename(from, to);
      await refreshShell();
      if (current === from) setCurrent(to);
      else if (current?.startsWith(from + "/")) setCurrent(to + current.slice(from.length));
    } catch (e) {
      fail(String(e));
    }
  }, [current, fail, refreshShell]);

  const remove = useCallback(async (node: Node) => {
    setConfirm(null);
    try {
      await api.remove(node.path);
      if (current === node.path || current?.startsWith(node.path + "/")) setCurrent(null);
      await refreshShell();
    } catch (e) {
      fail(String(e));
    }
  }, [current, fail, refreshShell]);

  const cycleTheme = () => chooseTheme(THEMES[(THEMES.indexOf(theme) + 1) % THEMES.length]);

  return (
    <div className="flex h-full bg-surface text-ink">
      <Sidebar
        tree={tree}
        workplans={workplans}
        labels={labels}
        weekHours={weekHours}
        current={current ?? ""}
        renaming={renaming}
        setRenaming={setRenaming}
        workplanFolder={info?.workplanDir.split(/[\\/]/).pop() ?? "Workplans"}
        theme={theme}
        version={info?.version ?? "dev"}
        onOpen={open}
        onSearch={(q) => void search(q)}
        onLabel={(n) => void byLabel(n)}
        onNewNote={(f) => void newNote(f)}
        onNewFolder={(f) => void newFolder(f)}
        onRename={(from, name) => void rename(from, name)}
        onDelete={(node) => setConfirm({ node })}
        onCycleTheme={cycleTheme}
        onOpenSettings={() => setSettingsOpen(true)}
        onOpenAbout={() => setAbout(true)}
      />

      {hits !== null ? (
        <main className="min-w-0 flex-1 overflow-y-auto px-8 py-5">
          <h1 className="mb-3 text-[15px] font-semibold">{hits.length} {hits.length === 1 ? "result" : "results"}</h1>
          <div className="space-y-0.5">
            {hits.map((hit) => (
              <button key={hit.path} type="button" onClick={() => open(hit.path)} className="block w-full rounded-md px-3 py-2 text-left hover:bg-surface-raised">
                <div className="font-mono text-[13px] text-accent">{hit.path}</div>
                {hit.snippet && <div className="text-xs text-ink-muted">{hit.snippet}</div>}
              </button>
            ))}
            {hits.length === 0 && <p className="text-sm text-ink-muted">Nothing matched.</p>}
          </div>
        </main>
      ) : current ? (
        <NoteView path={current} dark={dark} reloadToken={reloadToken} onShellChanged={() => void refreshShell()} onError={fail} />
      ) : (
        <main className="flex min-w-0 flex-1 items-center justify-center p-6">
          <p className="text-sm text-ink-faint">Pick a note, or press ＋ to make one.</p>
        </main>
      )}

      <AboutDialog open={about} info={info} onClose={() => setAbout(false)} />
      <SettingsDialog
        open={settingsOpen}
        onClose={() => setSettingsOpen(false)}
        theme={theme}
        onTheme={chooseTheme}
        onError={fail}
        onVaultChanged={() => { void refreshShell(); setReloadToken((n) => n + 1); }}
      />
      <ConfirmDialog
        open={confirm !== null}
        title={confirm?.node.isFolder ? "Delete folder?" : "Delete note?"}
        message={
          confirm?.node.isFolder
            ? `"${confirm.node.path}" and everything inside it will be deleted. This cannot be undone.`
            : `"${confirm?.node.path}" will be deleted. This cannot be undone.`
        }
        confirmLabel="Delete"
        danger
        onConfirm={() => confirm && void remove(confirm.node)}
        onCancel={() => setConfirm(null)}
      />

      {error && (
        <div role="alert" className="fixed bottom-4 right-4 z-50 max-w-md rounded-md border border-danger/40 bg-surface-raised p-3 text-xs text-danger shadow-lg">
          {error}
          <button type="button" onClick={() => setError(null)} className="ml-3 underline">dismiss</button>
        </div>
      )}
    </div>
  );
}

function findNode(tree: Node | null, path: string): Node | undefined {
  if (!tree) return undefined;
  for (const c of tree.children ?? []) {
    if (c.path === path) return c;
    const deeper = findNode(c, path);
    if (deeper) return deeper;
  }
  return undefined;
}
