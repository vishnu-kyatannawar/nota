// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

/**
 * Tiptap is mocked rather than mounted: the regression this pins is about when
 * the toolbar touches the editor, not about what ProseMirror does with it, and
 * a real editor in jsdom would only add flake.
 */
const chain = vi.fn();
const editor = {
  chain: () => {
    chain();
    return proxy;
  },
  isActive: () => false,
  getMarkdown: () => "",
  commands: { setContent: vi.fn() },
};
// Every chained command returns the chain, so a call records itself and moves on.
const calls: string[] = [];
const proxy: Record<string, (...a: unknown[]) => unknown> = new Proxy({} as never, {
  get: (_t, name: string) => (...a: unknown[]) => {
    calls.push(name + (a.length ? `(${JSON.stringify(a[0])})` : ""));
    return name === "run" ? true : proxy;
  },
});

vi.mock("@tiptap/react", () => ({
  useEditor: () => editor,
  useEditorState: ({ selector }: { selector: (a: unknown) => unknown }) => selector({ editor }),
  EditorContent: () => <div data-testid="editor-content" />,
}));
vi.mock("@tiptap/starter-kit", () => ({ default: { configure: () => ({}) } }));
vi.mock("@tiptap/markdown", () => ({ Markdown: {} }));
vi.mock("@tiptap/extension-code-block-lowlight", () => ({ default: { configure: () => ({}) } }));
vi.mock("@tiptap/extension-placeholder", () => ({ default: { configure: () => ({}) } }));
vi.mock("lowlight", () => ({ common: {}, createLowlight: () => ({}) }));

const { NotesEditor } = await import("./NotesEditor");

afterEach(() => {
  cleanup();
  chain.mockClear();
  calls.length = 0;
});

describe("NotesEditor toolbar", () => {
  it("does not touch the editor while rendering", () => {
    // A chain built at render scope runs focus() immediately, which is what
    // used to yank the caret out of an item row on every keystroke.
    const { rerender } = render(<NotesEditor value="" onChange={() => {}} />);
    expect(chain).not.toHaveBeenCalled();

    rerender(<NotesEditor value="x" onChange={() => {}} />);
    expect(chain).not.toHaveBeenCalled();
  });

  it("leaves focus where it was when the notes area re-renders", () => {
    render(
      <>
        <input data-testid="item" />
        <NotesEditor value="" onChange={() => {}} />
      </>,
    );
    const item = screen.getByTestId("item") as HTMLInputElement;
    item.focus();
    expect(document.activeElement).toBe(item);

    fireEvent.input(item, { target: { value: "typing" } });
    expect(document.activeElement).toBe(item);
    expect(chain).not.toHaveBeenCalled();
  });

  it("still focuses and applies the command when a button is pressed", () => {
    render(<NotesEditor value="" onChange={() => {}} />);
    fireEvent.mouseDown(screen.getByTitle("Bold (Ctrl+B)"));
    expect(chain).toHaveBeenCalledTimes(1);
    expect(calls).toEqual(["focus", "toggleBold", "run"]);
  });

  it("applies a heading level through its own chain", () => {
    render(<NotesEditor value="" onChange={() => {}} />);
    fireEvent.mouseDown(screen.getByTitle("Heading 2"));
    expect(calls).toEqual(["focus", 'toggleHeading({"level":2})', "run"]);
  });
});
