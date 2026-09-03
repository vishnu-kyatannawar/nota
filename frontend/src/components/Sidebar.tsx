import { useEffect, useState } from "react";
import type { Label, Node, Workplan } from "../lib/api";
import type { Theme } from "../lib/theme";
import { ContextMenu, type MenuItem } from "./ContextMenu";
import { Logo } from "./Logo";

export type TreeActions = {
  onOpen: (path: string) => void;
  onNewNote: (parentFolder: string) => void;
  onNewFolder: (parentFolder: string) => void;
  onRename: (from: string, newName: string) => void;
  onDelete: (node: Node) => void;
};

type Props = TreeActions & {
  tree: Node | null;
  workplans: Workplan[];
  labels: Label[];
  weekHours: string;
  current: string;
  renaming: string | null;
  setRenaming: (path: string | null) => void;
  workplanFolder: string;
  theme: Theme;
  version: string;
  onSearch: (query: string) => void;
  onLabel: (name: string) => void;
  onCycleTheme: () => void;
  onOpenSettings: () => void;
  onOpenAbout: () => void;
};

const EXPANDED_KEY = "nota.expanded";

function loadExpanded(): Set<string> {
  try {
    return new Set(JSON.parse(localStorage.getItem(EXPANDED_KEY) ?? "[]"));
  } catch {
    return new Set();
  }
}

export function Sidebar(props: Props) {
  const { tree, workplans, labels, weekHours, current, theme, version } = props;
  const [query, setQuery] = useState("");
  const [expanded, setExpanded] = useState<Set<string>>(loadExpanded);
  const [menu, setMenu] = useState<{ x: number; y: number; items: MenuItem[] } | null>(null);

  useEffect(() => {
    try {
      localStorage.setItem(EXPANDED_KEY, JSON.stringify([...expanded]));
    } catch {
      /* storage may be unavailable; expansion state is a convenience only */
    }
  }, [expanded]);

  const toggle = (path: string) =>
    setExpanded((s) => {
      const n = new Set(s);
      if (n.has(path)) n.delete(path);
      else n.add(path);
      return n;
    });

  const openMenu = (e: React.MouseEvent, node: Node | null) => {
    e.preventDefault();
    e.stopPropagation();
    const folder = node ? (node.isFolder ? node.path : parentOf(node.path)) : "";
    const reserved = node?.isFolder && node.path === props.workplanFolder;
    const items: MenuItem[] = [
      { label: "New note", onSelect: () => props.onNewNote(folder) },
      { label: "New folder", onSelect: () => props.onNewFolder(folder) },
    ];
    if (node) {
      items.push(
        { label: "Rename", onSelect: () => props.setRenaming(node.path), disabled: reserved },
        { label: node.isFolder ? "Delete folder…" : "Delete note…", onSelect: () => props.onDelete(node), danger: true, disabled: reserved },
      );
    }
    setMenu({ x: e.clientX, y: e.clientY, items });
  };

  const themeLabel = { system: "System theme", light: "Light theme", dark: "Dark theme" }[theme];
  const themeGlyph = { system: "◐", light: "☀", dark: "☾" }[theme];

  return (
    <aside className="flex w-64 shrink-0 flex-col border-r border-border bg-surface-raised">
      <div className="flex items-center gap-2 px-4 pt-4 pb-2">
        <Logo size={20} />
        <span className="text-[13px] font-semibold tracking-tight">Nota</span>
      </div>

      <div className="px-3 pb-2">
        <input
          value={query}
          onChange={(e) => { setQuery(e.target.value); props.onSearch(e.target.value); }}
          placeholder="Search"
          aria-label="Search notes"
          className="w-full rounded-md border border-border bg-surface px-2.5 py-1.5 text-[13px] outline-none placeholder:text-ink-faint focus:border-accent"
        />
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto px-2 pb-2" onContextMenu={(e) => openMenu(e, null)}>
        <Section
          title="Workplans"
          trailing={<span className="font-mono text-[10px] text-ink-muted">{weekHours} this week</span>}
        >
          {workplans.slice(0, 14).map((w) => (
            <button
              key={w.path}
              type="button"
              onClick={() => props.onOpen(w.path)}
              className={`flex w-full items-center gap-2 rounded-md px-2 py-1 text-left font-mono text-[12px] ${
                w.path === current ? "bg-accent-soft text-accent" : "text-ink-muted hover:bg-surface-sunken hover:text-ink"
              }`}
            >
              <span>{w.date}</span>
              <span className={`ml-auto ${w.dayType && w.dayType !== "work" ? "text-ink-faint" : ""}`}>{w.hours}</span>
              {w.open > 0 && <span className="w-4 text-right text-[10px] text-ink-faint">{w.open}</span>}
            </button>
          ))}
          {workplans.length === 0 && <Empty>No workplans yet</Empty>}
        </Section>

        <Section
          title="Notes"
          trailing={
            <span className="flex gap-0.5">
              <IconButton label="New note" onClick={() => props.onNewNote("")}>＋</IconButton>
              <IconButton label="New folder" onClick={() => props.onNewFolder("")}>▣</IconButton>
            </span>
          }
        >
          {tree?.children?.map((node) => (
            <TreeNode
              key={node.path}
              node={node}
              depth={0}
              current={current}
              expanded={expanded}
              toggle={toggle}
              renaming={props.renaming}
              setRenaming={props.setRenaming}
              onOpen={props.onOpen}
              onRename={props.onRename}
              onMenu={openMenu}
            />
          ))}
          {!tree?.children?.length && <Empty>Right-click or press ＋ to add a note or folder</Empty>}
        </Section>

        <Section title="Labels">
          <div className="flex flex-wrap gap-1 px-1">
            {labels.slice(0, 40).map((l) => (
              <button
                key={l.name}
                type="button"
                onClick={() => props.onLabel(l.name)}
                className="rounded-full bg-accent-soft px-2 py-0.5 text-[11px] text-accent hover:opacity-80"
              >
                #{l.name} <span className="opacity-60">{l.count}</span>
              </button>
            ))}
            {labels.length === 0 && <Empty>Type #label on any item</Empty>}
          </div>
        </Section>
      </div>

      <div className="flex items-center gap-1 border-t border-border px-2 py-2">
        <IconButton label={themeLabel} onClick={props.onCycleTheme}>{themeGlyph}</IconButton>
        <IconButton label="Settings" onClick={props.onOpenSettings}>⚙</IconButton>
        <button
          type="button"
          onClick={props.onOpenAbout}
          className="ml-auto rounded-md px-2 py-1 font-mono text-[11px] text-ink-faint hover:bg-surface-sunken hover:text-ink-muted"
        >
          v{version}
        </button>
      </div>

      {menu && <ContextMenu x={menu.x} y={menu.y} items={menu.items} onClose={() => setMenu(null)} />}
    </aside>
  );
}

function parentOf(path: string): string {
  const i = path.lastIndexOf("/");
  return i < 0 ? "" : path.slice(0, i);
}

function Section({ title, trailing, children }: { title: string; trailing?: React.ReactNode; children: React.ReactNode }) {
  return (
    <div className="mb-3">
      <div className="mb-1 flex items-center px-2 text-[10px] font-semibold uppercase tracking-[0.12em] text-ink-faint">
        {title}
        <span className="ml-auto">{trailing}</span>
      </div>
      {children}
    </div>
  );
}

function IconButton({ label, onClick, children }: { label: string; onClick: () => void; children: React.ReactNode }) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={label}
      title={label}
      className="flex h-6 w-6 items-center justify-center rounded-md text-[13px] text-ink-muted hover:bg-surface-sunken hover:text-ink"
    >
      {children}
    </button>
  );
}

function Empty({ children }: { children: React.ReactNode }) {
  return <p className="px-2 py-1 text-[12px] text-ink-faint">{children}</p>;
}

type TreeNodeProps = {
  node: Node;
  depth: number;
  current: string;
  expanded: Set<string>;
  toggle: (path: string) => void;
  renaming: string | null;
  setRenaming: (path: string | null) => void;
  onOpen: (path: string) => void;
  onRename: (from: string, newName: string) => void;
  onMenu: (e: React.MouseEvent, node: Node) => void;
};

function TreeNode(p: TreeNodeProps) {
  const { node, depth } = p;
  const isOpen = p.expanded.has(node.path);
  const active = node.path === p.current;
  const pad = depth * 12 + 6;

  if (p.renaming === node.path) {
    return <RenameInput node={node} pad={pad} onDone={(name) => { p.setRenaming(null); if (name) p.onRename(node.path, name); }} />;
  }

  const row = (
    <button
      type="button"
      onClick={() => (node.isFolder ? p.toggle(node.path) : p.onOpen(node.path))}
      onContextMenu={(e) => p.onMenu(e, node)}
      onDoubleClick={() => node.isFolder && p.setRenaming(node.path)}
      style={{ paddingLeft: pad }}
      className={`group flex w-full items-center gap-1.5 rounded-md py-1 pr-1 text-left text-[13px] ${
        active ? "bg-accent-soft text-accent" : "text-ink-muted hover:bg-surface-sunken hover:text-ink"
      }`}
    >
      {node.isFolder ? (
        <span className="w-3 text-center text-[10px] text-ink-faint">{isOpen ? "▾" : "▸"}</span>
      ) : (
        <span className="w-3 text-center text-[10px] text-ink-faint">·</span>
      )}
      <span className="truncate">{node.name}</span>
      <span
        role="button"
        tabIndex={-1}
        aria-label="More"
        onClick={(e) => p.onMenu(e, node)}
        className="ml-auto hidden rounded px-1 text-ink-faint group-hover:inline hover:text-ink"
      >
        ⋯
      </span>
    </button>
  );

  return (
    <div>
      {row}
      {node.isFolder && isOpen && node.children?.map((c) => <TreeNode key={c.path} {...p} node={c} depth={depth + 1} />)}
    </div>
  );
}

function RenameInput({ node, pad, onDone }: { node: Node; pad: number; onDone: (name: string | null) => void }) {
  const [value, setValue] = useState(node.name);
  return (
    <input
      autoFocus
      value={value}
      onChange={(e) => setValue(e.target.value)}
      onFocus={(e) => e.target.select()}
      onBlur={() => onDone(value.trim() && value.trim() !== node.name ? value.trim() : null)}
      onKeyDown={(e) => {
        if (e.key === "Enter") onDone(value.trim() && value.trim() !== node.name ? value.trim() : null);
        if (e.key === "Escape") onDone(null);
      }}
      aria-label={`Rename ${node.name}`}
      style={{ marginLeft: pad }}
      className="my-0.5 w-[calc(100%-1rem)] rounded-md border border-accent bg-surface px-1.5 py-0.5 text-[13px] outline-none"
    />
  );
}
