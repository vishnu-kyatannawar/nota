import { useCallback, useEffect, useState } from "react";
import { Browser, Events } from "@wailsio/runtime";
import type { Hit, Info, Label, Node, TrashEntry, UpdateCheck, UpdateState, Workplan } from "./lib/api";
import { api } from "./lib/api";
import { applyTheme, isTheme, resolvedTheme, THEMES, type Theme } from "./lib/theme";
import { applyFonts, DEFAULT_FONTS, normaliseFonts, type Fonts } from "./lib/fonts";
import { findNode } from "./lib/tree";
import { useExpanded } from "./lib/expanded";
import { Sidebar } from "./components/Sidebar";
import { NoteView } from "./components/NoteView";
import { AboutDialog } from "./components/AboutDialog";
import { SettingsDialog } from "./components/SettingsDialog";
import { ConfirmDialog } from "./components/ConfirmDialog";
import { Banner } from "./components/Banner";
import { UpdateConsentDialog } from "./components/UpdateConsentDialog";

export default function App() {
  const [info, setInfo] = useState<Info | null>(null);
  const [theme, setTheme] = useState<Theme>("system");
  const [dark, setDark] = useState(false);
  const [fonts, setFonts] = useState<Fonts>(DEFAULT_FONTS);
  const [tree, setTree] = useState<Node | null>(null);
  const [workplans, setWorkplans] = useState<Workplan[]>([]);
  const [labels, setLabels] = useState<Label[]>([]);
  const [trash, setTrash] = useState<TrashEntry[]>([]);
  const [todayPath, setTodayPath] = useState<string | null>(null);
  const [weekHours, setWeekHours] = useState("00:00");
  const [current, setCurrent] = useState<string | null>(null);
  const [reloadToken, setReloadToken] = useState(0);
  const [hits, setHits] = useState<Hit[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [renaming, setRenaming] = useState<string | null>(null);
  const [about, setAbout] = useState(false);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [confirm, setConfirm] = useState<{ kind: "delete"; node: Node } | { kind: "forever"; entry: TrashEntry } | { kind: "empty" } | null>(null);

  const [update, setUpdate] = useState<UpdateState | null>(null);
  // Dismissing the update notice silences it for this session only.
  const [updateHidden, setUpdateHidden] = useState(false);

  const { expanded, toggle, expand, reveal, renamePrefix, forget } = useExpanded();

  // A rename box only exists as a row in the tree, so a path that has gone away
  // — deleted elsewhere, or created into a folder that never opened — must not
  // leave a rename pending on nothing. Derived, not synced: the stale state is
  // simply never rendered, and the next rename overwrites it.
  const renamingNode = renaming && findNode(tree, renaming) ? renaming : null;

  const fail = useCallback((m: string) => setError(m), []);

  const refreshShell = useCallback(async () => {
    try {
      const [t, w, l, h, tr] = await Promise.all([api.tree(), api.workplans(), api.labels(), api.hoursThisWeek(), api.listTrash()]);
      setTree(t);
      setWorkplans(w);
      setLabels(l);
      setWeekHours(h.hours);
      setTrash(tr);
    } catch (e) {
      fail(String(e));
    }
  }, [fail]);

  // A fresh closure here re-runs NoteView's reload effect on every App render,
  // which re-keys rows and resets the caret mid-word.
  const shellChanged = useCallback(() => void refreshShell(), [refreshShell]);

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
        const f = normaliseFonts(i.fonts);
        setFonts(f);
        applyFonts(f);
        const today = await api.ensureToday();
        await refreshShell();
        if (today) {
          setTodayPath(today);
          setCurrent(today);
        }
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
      if (ev?.data) {
        setTodayPath(ev.data);
        open(ev.data);
      }
    });
    Events.On("update:state", (ev) => {
      // The generated event type widens phase to string; narrow it here, the
      // same way api.ts casts its service results.
      const state = ev?.data as UpdateState | undefined;
      if (!state) return;
      setUpdate(state);
      setUpdateHidden(false);
    });
    return () => {
      Events.Off("note:changed");
      Events.Off("workplan:rolled");
      Events.Off("update:state");
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
      reveal(path);
      open(path);
      setRenaming(path);
    } catch (e) {
      fail(String(e));
    }
  }, [fail, open, refreshShell, reveal]);

  const newFolder = useCallback(async (folder: string) => {
    try {
      let name = "New folder";
      const siblings = new Set((folder ? findNode(tree, folder)?.children : tree?.children)?.map((c) => c.name) ?? []);
      for (let i = 2; siblings.has(name); i++) name = `New folder ${i}`;
      const path = `${folder ? folder + "/" : ""}${name}`;
      await api.createFolder(path);
      await refreshShell();
      reveal(path);
      expand(path);
      setRenaming(path);
    } catch (e) {
      fail(String(e));
    }
  }, [expand, fail, refreshShell, reveal, tree]);

  const answerUpdateConsent = useCallback((check: UpdateCheck) => {
    setInfo((i) => (i ? { ...i, updateCheck: check } : i));
    api.setUpdateCheck(check).catch((e) => fail(String(e)));
    // Saying yes should not mean waiting until tomorrow to find out.
    if (check === "auto") void api.checkUpdate(false).then(setUpdate).catch(() => {});
  }, [fail]);

  const installUpdate = useCallback(() => {
    void api.installUpdate().then(setUpdate).catch((e) => fail(String(e)));
  }, [fail]);

  const chooseFonts = useCallback((f: Fonts) => {
    setFonts(f);
    applyFonts(f);
    api.setFonts(f).catch((e) => fail(String(e)));
  }, [fail]);

  const rename = useCallback(async (from: string, newName: string) => {
    const parent = from.includes("/") ? from.slice(0, from.lastIndexOf("/") + 1) : "";
    const isNote = from.endsWith(".md");
    const clean = newName.replace(/[/\\]/g, "-").replace(/\.md$/i, "");
    const to = `${parent}${clean}${isNote ? ".md" : ""}`;
    if (!clean || to === from) return;
    try {
      await api.rename(from, to);
      renamePrefix(from, to);
      await refreshShell();
      if (current === from) setCurrent(to);
      else if (current?.startsWith(from + "/")) setCurrent(to + current.slice(from.length));
    } catch (e) {
      fail(String(e));
    }
  }, [current, fail, refreshShell, renamePrefix]);

  const remove = useCallback(async (node: Node) => {
    setConfirm(null);
    try {
      await api.remove(node.path);
      forget(node.path);
      if (current === node.path || current?.startsWith(node.path + "/")) setCurrent(null);
      await refreshShell();
    } catch (e) {
      fail(String(e));
    }
  }, [current, fail, forget, refreshShell]);

  const restore = useCallback(async (id: string) => {
    try {
      const path = await api.restore(id);
      await refreshShell();
      if (path.endsWith(".md")) open(path);
    } catch (e) {
      fail(String(e));
    }
  }, [fail, open, refreshShell]);

  const deleteForever = useCallback(async (entry: TrashEntry) => {
    setConfirm(null);
    try {
      await api.deleteForever(entry.id);
      await refreshShell();
    } catch (e) {
      fail(String(e));
    }
  }, [fail, refreshShell]);

  const emptyTrash = useCallback(async () => {
    setConfirm(null);
    try {
      await api.emptyTrash();
      await refreshShell();
    } catch (e) {
      fail(String(e));
    }
  }, [fail, refreshShell]);

  const cycleTheme = () => chooseTheme(THEMES[(THEMES.indexOf(theme) + 1) % THEMES.length]);

  return (
    <div className="flex h-full bg-surface text-ink">
      <Sidebar
        tree={tree}
        workplans={workplans}
        labels={labels}
        trash={trash}
        onRestore={(id) => void restore(id)}
        onDeleteForever={(entry) => setConfirm({ kind: "forever", entry })}
        onEmptyTrash={() => setConfirm({ kind: "empty" })}
        weekHours={weekHours}
        current={current ?? ""}
        renaming={renamingNode}
        expanded={expanded}
        onToggle={toggle}
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
        onDelete={(node) => setConfirm({ kind: "delete", node })}
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
        <NoteView
          path={current}
          dark={dark}
          reloadToken={reloadToken}
          allLabels={labels.map((l) => l.name)}
          todayPath={todayPath}
          onShellChanged={shellChanged}
          onError={fail}
        />
      ) : (
        <main className="flex min-w-0 flex-1 items-center justify-center p-6">
          <p className="text-sm text-ink-faint">Pick a page, or press ＋ to make one.</p>
        </main>
      )}

      <AboutDialog open={about} info={info} onClose={() => setAbout(false)} />
      <SettingsDialog
        open={settingsOpen}
        onClose={() => setSettingsOpen(false)}
        theme={theme}
        onTheme={chooseTheme}
        fonts={fonts}
        onFonts={chooseFonts}
        onError={fail}
        onVaultChanged={() => { void refreshShell(); setReloadToken((n) => n + 1); }}
        updateCheck={info?.updateCheck ?? "ask"}
        onUpdateCheck={answerUpdateConsent}
      />
      <ConfirmDialog
        open={confirm !== null}
        title={
          confirm?.kind === "delete" ? (confirm.node.isFolder ? "Move folder to trash?" : "Move page to trash?")
          : confirm?.kind === "forever" ? "Delete forever?"
          : "Empty the trash?"
        }
        message={
          confirm?.kind === "delete"
            ? `"${confirm.node.path}"${confirm.node.isFolder ? " and everything inside it" : ""} will move to Trash, where it is kept for 30 days and can be restored.`
            : confirm?.kind === "forever"
              ? `"${confirm.entry.path}" will be deleted permanently. This cannot be undone.`
              : `Everything in the trash (${trash.length}) will be deleted permanently. This cannot be undone.`
        }
        confirmLabel={confirm?.kind === "delete" ? "Move to trash" : "Delete forever"}
        danger
        onConfirm={() => {
          if (!confirm) return;
          if (confirm.kind === "delete") void remove(confirm.node);
          else if (confirm.kind === "forever") void deleteForever(confirm.entry);
          else void emptyTrash();
        }}
        onCancel={() => setConfirm(null)}
      />

      <UpdateConsentDialog
        open={info?.updateCheck === "ask"}
        onAnswer={answerUpdateConsent}
      />

      {error && <Banner tone="danger" onDismiss={() => setError(null)}>{error}</Banner>}

      {!error && !updateHidden && update && updateBanner(update, installUpdate, () => setUpdateHidden(true))}
    </div>
  );
}

/** The corner notice for whatever the updater is doing, or nothing. */
function updateBanner(u: UpdateState, install: () => void, dismiss: () => void) {
  switch (u.phase) {
    case "available":
      return u.canInstall ? (
        <Banner tone="accent" action={{ label: "Install", onClick: install }} onDismiss={dismiss}>
          Nota {u.version} is available.
        </Banner>
      ) : (
        // Not our binary to replace — a package-managed or read-only install.
        <Banner tone="accent" action={{ label: "Open releases", onClick: () => void Browser.OpenURL(u.releaseUrl) }} onDismiss={dismiss}>
          Nota {u.version} is available. This copy was not installed by you, so update it the way you installed it.
        </Banner>
      );
    case "downloading":
      return (
        <Banner tone="accent" action={{ label: `${u.percent}%`, onClick: () => {}, busy: true }}>
          Downloading Nota {u.version}…
        </Banner>
      );
    case "ready":
      return (
        <Banner tone="accent" onDismiss={dismiss}>
          Nota {u.version} is installed. Restart Nota to use it.
        </Banner>
      );
    case "failed":
      return <Banner tone="danger" onDismiss={dismiss}>Update failed: {u.message}</Banner>;
    default:
      return null;
  }
}
