// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { Sidebar } from "./Sidebar";
import type { Node } from "../lib/api";

const tree: Node = {
  name: "vault",
  path: "",
  isFolder: true,
  children: [
    { name: "Projects", path: "Projects", isFolder: true, children: [{ name: "api", path: "Projects/api.md", isFolder: false }] },
    { name: "scratch", path: "scratch.md", isFolder: false },
  ],
};

afterEach(cleanup);

function show(expanded: string[] = []) {
  const props = {
    tree,
    workplans: [],
    labels: [],
    trash: [],
    weekHours: "00:00",
    current: "",
    renaming: null,
    workplanFolder: "Workplans",
    theme: "light" as const,
    version: "4.1.0",
    expanded: new Set(expanded),
    onToggle: vi.fn(),
    onOpen: vi.fn(),
    onNewNote: vi.fn(),
    onNewFolder: vi.fn(),
    onRename: vi.fn(),
    onDelete: vi.fn(),
    onRestore: vi.fn(),
    onDeleteForever: vi.fn(),
    onEmptyTrash: vi.fn(),
    setRenaming: vi.fn(),
    onSearch: vi.fn(),
    onLabel: vi.fn(),
    onCycleTheme: vi.fn(),
    onOpenSettings: vi.fn(),
    onOpenAbout: vi.fn(),
  };
  render(<Sidebar {...props} />);
  return props;
}

const labels = () => screen.getAllByRole("menuitem").map((n) => n.textContent);

describe("tree context menu", () => {
  it("offers nothing to put inside a page", () => {
    show();
    fireEvent.contextMenu(screen.getByText("scratch"));
    expect(labels()).toEqual(["Rename", "Delete page…"]);
  });

  it("offers a new page or folder inside a folder", () => {
    show();
    fireEvent.contextMenu(screen.getByText("Projects"));
    expect(labels()).toEqual(["New page", "New folder", "Rename", "Delete folder…"]);
  });

  it("creates into the folder that was right-clicked", () => {
    const props = show();
    fireEvent.contextMenu(screen.getByText("Projects"));
    fireEvent.click(screen.getByText("New page"));
    expect(props.onNewNote).toHaveBeenCalledWith("Projects");
  });
});

describe("expansion", () => {
  it("shows a folder's children only when it is expanded", () => {
    cleanup();
    show();
    expect(screen.queryByText("api")).toBeNull();
    cleanup();
    show(["Projects"]);
    expect(screen.getByText("api")).toBeTruthy();
  });

  it("asks the owner to toggle rather than tracking it itself", () => {
    const props = show();
    fireEvent.click(screen.getByText("Projects"));
    expect(props.onToggle).toHaveBeenCalledWith("Projects");
  });
});
