import { useState } from "react";
import type { Label, Node, Workplan } from "../lib/api";

type Props = {
  tree: Node | null;
  workplans: Workplan[];
  labels: Label[];
  weekHours: string;
  current: string;
  onOpen: (path: string) => void;
  onSearch: (query: string) => void;
  onLabel: (name: string) => void;
};

const dayTypeBadge: Record<string, string> = {
  weekend: "text-ink-muted/60",
  leave: "text-sky-400",
  holiday: "text-violet-400",
};

export function Sidebar({
  tree,
  workplans,
  labels,
  weekHours,
  current,
  onOpen,
  onSearch,
  onLabel,
}: Props) {
  const [query, setQuery] = useState("");

  return (
    <aside className="flex w-64 shrink-0 flex-col border-r border-surface-border bg-surface-raised">
      <div className="border-b border-surface-border p-3">
        <input
          value={query}
          onChange={(e) => {
            setQuery(e.target.value);
            onSearch(e.target.value);
          }}
          placeholder="Search notes"
          aria-label="Search notes"
          className="w-full rounded border border-surface-border bg-surface px-2 py-1.5 text-sm outline-none placeholder:text-ink-muted/60 focus:border-accent/60"
        />
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto p-3">
        <Heading>
          Workplans
          <span className="ml-auto font-mono text-[10px] text-amber-400">{weekHours} this week</span>
        </Heading>
        <div className="mb-4 space-y-0.5">
          {workplans.slice(0, 14).map((w) => (
            <button
              key={w.path}
              type="button"
              onClick={() => onOpen(w.path)}
              className={`flex w-full items-center gap-2 rounded px-2 py-1 text-left font-mono text-xs ${
                w.path === current ? "bg-accent/20 text-accent" : "text-ink-muted hover:bg-white/5"
              }`}
            >
              <span>{w.date}</span>
              <span className={`ml-auto ${dayTypeBadge[w.dayType] ?? "text-amber-400/80"}`}>
                {w.hours}
              </span>
              {w.open > 0 && <span className="text-[10px] text-ink-muted/70">{w.open}</span>}
            </button>
          ))}
          {workplans.length === 0 && <Empty>No workplans yet</Empty>}
        </div>

        <Heading>Vault</Heading>
        <div className="mb-4">
          {tree?.children?.map((node) => (
            <TreeNode key={node.path} node={node} depth={0} current={current} onOpen={onOpen} />
          ))}
          {!tree?.children?.length && <Empty>Empty vault</Empty>}
        </div>

        <Heading>Labels</Heading>
        <div className="flex flex-wrap gap-1">
          {labels.slice(0, 30).map((l) => (
            <button
              key={l.name}
              type="button"
              onClick={() => onLabel(l.name)}
              className="rounded-full bg-accent/12 px-1.5 py-0.5 text-[11px] text-accent hover:bg-accent/25"
            >
              #{l.name}
              <span className="ml-1 text-accent/60">{l.count}</span>
            </button>
          ))}
          {labels.length === 0 && <Empty>No labels yet</Empty>}
        </div>
      </div>
    </aside>
  );
}

function Heading({ children }: { children: React.ReactNode }) {
  return (
    <div className="mb-1.5 flex items-center text-[10px] font-bold uppercase tracking-widest text-ink-muted/60">
      {children}
    </div>
  );
}

function Empty({ children }: { children: React.ReactNode }) {
  return <p className="px-2 py-1 text-xs text-ink-muted/50">{children}</p>;
}

function TreeNode({
  node,
  depth,
  current,
  onOpen,
}: {
  node: Node;
  depth: number;
  current: string;
  onOpen: (path: string) => void;
}) {
  const [open, setOpen] = useState(depth === 0);

  if (!node.isFolder) {
    return (
      <button
        type="button"
        onClick={() => onOpen(node.path)}
        style={{ paddingLeft: depth * 12 + 8 }}
        className={`block w-full truncate rounded py-1 pr-2 text-left text-xs ${
          node.path === current ? "bg-accent/20 text-accent" : "text-ink-muted hover:bg-white/5"
        }`}
      >
        {node.name}
      </button>
    );
  }

  return (
    <div>
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        style={{ paddingLeft: depth * 12 + 4 }}
        className="flex w-full items-center gap-1 rounded py-1 pr-2 text-left text-xs text-ink hover:bg-white/5"
      >
        <span className="text-ink-muted/60">{open ? "▾" : "▸"}</span>
        <span className="truncate">{node.name}</span>
      </button>
      {open &&
        node.children?.map((child) => (
          <TreeNode key={child.path} node={child} depth={depth + 1} current={current} onOpen={onOpen} />
        ))}
    </div>
  );
}
