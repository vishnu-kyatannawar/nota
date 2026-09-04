import { describe, expect, it } from "vitest";
import { ancestorsOf, findNode } from "./tree";
import type { Node } from "./api";

describe("ancestorsOf", () => {
  it("lists every folder above the path, outermost first", () => {
    expect(ancestorsOf("A/B/c.md")).toEqual(["A", "A/B"]);
  });

  it("has no ancestors at the root", () => {
    expect(ancestorsOf("c.md")).toEqual([]);
    expect(ancestorsOf("")).toEqual([]);
  });

  it("treats a folder path the same as a note path", () => {
    expect(ancestorsOf("A/B")).toEqual(["A"]);
  });
});

describe("findNode", () => {
  const tree: Node = {
    name: "vault",
    path: "",
    isFolder: true,
    children: [
      { name: "A", path: "A", isFolder: true, children: [{ name: "c", path: "A/c.md", isFolder: false }] },
      { name: "d", path: "d.md", isFolder: false },
    ],
  };

  it("finds a nested node", () => {
    expect(findNode(tree, "A/c.md")?.name).toBe("c");
  });

  it("finds a top-level node", () => {
    expect(findNode(tree, "d.md")?.name).toBe("d");
  });

  it("returns undefined for a missing path or a missing tree", () => {
    expect(findNode(tree, "nope.md")).toBeUndefined();
    expect(findNode(null, "d.md")).toBeUndefined();
  });
});
