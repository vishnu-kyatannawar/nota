/**
 * Which folders are open in the sidebar tree. This lives above the Sidebar
 * because App is what creates, renames and deletes nodes, and each of those
 * has to keep the set honest: a new node is worthless if its parent stays
 * shut, and a renamed folder whose old path lingers here silently collapses.
 */

import { useCallback, useEffect, useState } from "react";
import { ancestorsOf } from "./tree";

const KEY = "nota.expanded";

function load(): Set<string> {
  try {
    return new Set(JSON.parse(localStorage.getItem(KEY) ?? "[]"));
  } catch {
    return new Set();
  }
}

export type Expanded = {
  expanded: Set<string>;
  /** Opens a shut folder, or shuts an open one. */
  toggle: (path: string) => void;
  /** Opens a folder, whether or not it was already open. */
  expand: (path: string) => void;
  /** Opens everything above `path` so a node at it is on screen. */
  reveal: (path: string) => void;
  /** Follows a rename, including every descendant folder. */
  renamePrefix: (from: string, to: string) => void;
  /** Drops a path and everything under it, so deletions do not accumulate. */
  forget: (path: string) => void;
};

export function useExpanded(): Expanded {
  const [expanded, setExpanded] = useState<Set<string>>(load);

  useEffect(() => {
    try {
      localStorage.setItem(KEY, JSON.stringify([...expanded]));
    } catch {
      /* storage may be unavailable; expansion state is a convenience only */
    }
  }, [expanded]);

  const toggle = useCallback((path: string) => {
    setExpanded((s) => {
      const n = new Set(s);
      if (n.has(path)) n.delete(path);
      else n.add(path);
      return n;
    });
  }, []);

  const expand = useCallback((path: string) => {
    if (!path) return;
    setExpanded((s) => (s.has(path) ? s : new Set(s).add(path)));
  }, []);

  const reveal = useCallback((path: string) => {
    const wanted = ancestorsOf(path);
    if (wanted.length === 0) return;
    setExpanded((s) => {
      if (wanted.every((a) => s.has(a))) return s;
      const n = new Set(s);
      for (const a of wanted) n.add(a);
      return n;
    });
  }, []);

  const renamePrefix = useCallback((from: string, to: string) => {
    setExpanded((s) => {
      const n = new Set<string>();
      let changed = false;
      for (const p of s) {
        if (p === from) {
          n.add(to);
          changed = true;
        } else if (p.startsWith(from + "/")) {
          n.add(to + p.slice(from.length));
          changed = true;
        } else {
          n.add(p);
        }
      }
      return changed ? n : s;
    });
  }, []);

  const forget = useCallback((path: string) => {
    setExpanded((s) => {
      const n = new Set([...s].filter((p) => p !== path && !p.startsWith(path + "/")));
      return n.size === s.size ? s : n;
    });
  }, []);

  return { expanded, toggle, expand, reveal, renamePrefix, forget };
}
