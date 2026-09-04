/**
 * Path arithmetic on the vault tree. Paths are vault-relative and always use
 * forward slashes, so these are plain string operations — the folder tree the
 * Go side hands over is only needed to look a node up.
 */

import type { Node } from "./api";

/** Every folder above this path, outermost first: "A/B/c.md" -> ["A", "A/B"]. */
export function ancestorsOf(path: string): string[] {
  const out: string[] = [];
  for (let i = path.indexOf("/"); i >= 0; i = path.indexOf("/", i + 1)) out.push(path.slice(0, i));
  return out;
}

/** Finds a node anywhere in the tree by its path. */
export function findNode(tree: Node | null, path: string): Node | undefined {
  if (!tree) return undefined;
  for (const c of tree.children ?? []) {
    if (c.path === path) return c;
    const deeper = findNode(c, path);
    if (deeper) return deeper;
  }
  return undefined;
}
