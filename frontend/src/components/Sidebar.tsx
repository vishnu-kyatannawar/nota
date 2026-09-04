import { useRef, useState } from "react";
import type { Label, Node, TrashEntry, Workplan } from "../lib/api";
import type { Theme } from "../lib/theme";
import { ContextMenu, type MenuItem } from "./ContextMenu";
import { Logo } from "./Logo";
import { Icon } from "./Icon";

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
  trash: TrashEntry[];
  onRestore: (id: string) => void;
  onDeleteForever: (entry: TrashEntry) => void;
  onEmptyTrash: () => void;
  weekHours: string;
  current: string;
  renaming: string | null;
  setRenaming: (path: string | null) => void;
  expanded: Set<string>;
  onToggle: (path: string) => void;
  workplanFolder: string;
  theme: Theme;
  version: string;
  onSearch: (query: string) => void;
  onLabel: (name: string) => void;
  onCycleTheme: () => void;
  onOpenSettings: () => void;
  onOpenAbout: () => void;
};

export function Sidebar(props: Props) {
  const { tree, workplans, labels, trash, weekHours, current, theme, version, expanded } = props;
  const [query, setQuery] = useState("");
  const [trashOpen, setTrashOpen] = useState(false);
  const [menu, setMenu] = useState<{ x: number; y: number; items: MenuItem[] } | null>(null);

  const openMenu = (e: React.MouseEvent, node: Node | null) => {
    e.preventDefault();
    e.stopPropagation();
    const reserved = node?.isFolder && node.path === props.workplanFolder;
    const items: MenuItem[] = [];
    // A page holds items and notes, not other pages, so only a folder — or the
    // empty space, meaning the vault root — offers to put something inside.
    if (!node || node.isFolder) {
      const folder = node ? node.path : "";
      items.push(
        { label: "New page", onSelect: () => props.onNewNote(folder) },
        { label: "New folder", onSelect: () => props.onNewFolder(folder) },
      );
    }
    if (node) {
      items.push(
        { label: "Rename", onSelect: () => props.setRenaming(node.path), disabled: reserved },
        { label: node.isFolder ? "Delete folder…" : "Delete page…", onSelect: () => props.onDelete(node), danger: true, disabled: reserved },
      );
    }
    setMenu({ x: e.clientX, y: e.clientY, items });
  };

  const themeLabel = { system: "Theme: follows the system", light: "Theme: light", dark: "Theme: dark" }[theme];
  const themeIcon = ({ system: "monitor", light: "sun", dark: "moon" } as const)[theme];

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
          aria-label="Search pages"
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
          title="Pages"
          trailing={
            <span className="flex gap-0.5">
              <IconButton label="New page" onClick={() => props.onNewNote("")}>＋</IconButton>
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
              toggle={props.onToggle}
              renaming={props.renaming}
              setRenaming={props.setRenaming}
              onOpen={props.onOpen}
              onRename={props.onRename}
              onMenu={openMenu}
            />
          ))}
          {!tree?.children?.length && <Empty>Right-click or press ＋ to add a page or folder</Empty>}
        </Section>

        <Section
          title={
            <button type="button" onClick={() => setTrashOpen((v) => !v)} className="flex items-center gap-1 uppercase hover:text-ink-muted">
              <span className="w-3 text-center">{trashOpen ? "▾" : "▸"}</span>Trash{trash.length > 0 && <span className="ml-1 normal-case tracking-normal text-ink-faint">{trash.length}</span>}
            </button>
          }
          trailing={trashOpen && trash.length > 0 ? (
            <button type="button" onClick={props.onEmptyTrash} className="text-[10px] normal-case tracking-normal text-ink-faint hover:text-danger">Empty</button>
          ) : null}
        >
          {trashOpen && trash.map((e) => (
            <div key={e.id} className="group flex items-center gap-1.5 rounded-md px-2 py-1 text-[12px] text-ink-muted hover:bg-surface-sunken">
              <span className="w-3 text-center text-[10px] text-ink-faint">{e.isFolder ? "▣" : "·"}</span>
              <span className="min-w-0 flex-1 truncate" title={e.path}>{e.name.replace(/\.md$/, "")}</span>
              <span className="text-[10px] text-ink-faint">{ago(e.deletedAt)}</span>
              <button type="button" onClick={() => props.onRestore(e.id)} title="Restore" aria-label={`Restore ${e.path}`} className="hidden rounded px-1 text-accent group-hover:inline">↶</button>
              <button type="button" onClick={() => props.onDeleteForever(e)} title="Delete forever" aria-label={`Delete ${e.path} forever`} className="hidden rounded px-1 text-danger group-hover:inline">×</button>
            </div>
          ))}
          {trashOpen && trash.length === 0 && <Empty>Nothing in the trash. Deleted pages stay here for 30 days.</Empty>}
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
        <IconButton label={themeLabel} onClick={props.onCycleTheme}><Icon name={themeIcon} /></IconButton>
        <IconButton label="Settings" onClick={props.onOpenSettings}><Icon name="settings" /></IconButton>
        <button
          type="button"
          onClick={props.onOpenAbout}
          title="About Nota"
          className="ml-auto flex items-center gap-1.5 rounded-md px-2 py-1 font-mono text-[12px] text-ink-muted hover:bg-surface-sunken hover:text-ink"
        >
          <Icon name="info" size={14} />v{version}
        </button>
      </div>

      {menu && <ContextMenu x={menu.x} y={menu.y} items={menu.items} onClose={() => setMenu(null)} />}
    </aside>
  );
}

function ago(iso: string): string {
  const ms = Date.now() - new Date(iso).getTime();
  const d = Math.floor(ms / 86_400_000);
  if (d >= 1) return `${d}d`;
  const h = Math.floor(ms / 3_600_000);
  if (h >= 1) return `${h}h`;
  return `${Math.max(1, Math.floor(ms / 60_000))}m`;
}

function Section({ title, trailing, children }: { title: React.ReactNode; trailing?: React.ReactNode; children: React.ReactNode }) {
  return (
    <div className="mb-3">
      <div className="mb-1 flex items-center px-2 text-[11px] font-semibold uppercase tracking-[0.1em] text-ink-muted">
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
      className="flex h-7 w-7 items-center justify-center rounded-md text-[14px] text-ink-muted hover:bg-surface-sunken hover:text-ink"
    >
      {children}
    </button>
  );
}

function Empty({ children }: { children: React.ReactNode }) {
  return <p className="px-2 py-1 text-[12px] text-ink-muted">{children}</p>;
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
      onDoubleClick={() => p.setRenaming(node.path)}
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
  // Committing unmounts this input, and unmounting fires blur — so without a
  // guard every rename ran twice (the second failing on a path that no longer
  // existed) and Escape committed instead of cancelling.
  const finished = useRef(false);
  const finish = (name: string | null) => {
    if (finished.current) return;
    finished.current = true;
    onDone(name);
  };
  const changed = () => (value.trim() && value.trim() !== node.name ? value.trim() : null);

  return (
    <input
      autoFocus
      // Revealing a folder can put this row below the fold; autoFocus alone
      // does not reliably scroll it back.
      ref={(el) => el?.scrollIntoView({ block: "nearest" })}
      value={value}
      onChange={(e) => setValue(e.target.value)}
      onFocus={(e) => e.target.select()}
      onBlur={() => finish(changed())}
      onKeyDown={(e) => {
        if (e.key === "Enter") { e.preventDefault(); finish(changed()); }
        if (e.key === "Escape") { e.preventDefault(); finish(null); }
      }}
      aria-label={`Rename ${node.name}`}
      style={{ marginLeft: pad }}
      className="my-0.5 w-[calc(100%-1rem)] rounded-md border border-accent bg-surface px-1.5 py-0.5 text-[13px] outline-none"
    />
  );
}
