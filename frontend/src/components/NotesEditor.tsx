import { useEffect, useRef } from "react";
import { EditorContent, useEditor, useEditorState, type Editor } from "@tiptap/react";
import StarterKit from "@tiptap/starter-kit";
import { Markdown } from "@tiptap/markdown";
import CodeBlockLowlight from "@tiptap/extension-code-block-lowlight";
import Placeholder from "@tiptap/extension-placeholder";
import { common, createLowlight } from "lowlight";

const lowlight = createLowlight(common);

type Props = {
  value: string;
  onChange: (markdown: string) => void;
  onBlur?: () => void;
  placeholder?: string;
};

/**
 * The free-form notes area under a note's items. A WYSIWYG editor whose
 * document is markdown on disk: Tiptap reads the note's body and writes it
 * back through @tiptap/markdown. Only this section is ever re-serialised —
 * the item lines above it are never touched by this editor.
 */
export function NotesEditor({ value, onChange, onBlur, placeholder = "Notes…" }: Props) {
  const handlers = useRef({ onChange, onBlur });
  useEffect(() => {
    handlers.current = { onChange, onBlur };
  }, [onChange, onBlur]);

  const editor = useEditor({
    extensions: [
      StarterKit.configure({ codeBlock: false }),
      CodeBlockLowlight.configure({ lowlight }),
      Markdown,
      Placeholder.configure({ placeholder }),
    ],
    content: value,
    contentType: "markdown",
    onUpdate: ({ editor }) => handlers.current.onChange(editor.getMarkdown()),
    onBlur: () => handlers.current.onBlur?.(),
  });

  // Adopt a value that changed elsewhere — a note switch, or the file edited
  // on disk — without disturbing local typing when nothing actually differs.
  useEffect(() => {
    if (!editor) return;
    if (editor.getMarkdown().trim() === value.trim()) return;
    editor.commands.setContent(value, { contentType: "markdown", emitUpdate: false });
  }, [editor, value]);

  if (!editor) return null;
  return (
    <div className="nota-prose">
      <Toolbar editor={editor} />
      <EditorContent editor={editor} />
    </div>
  );
}

function Toolbar({ editor }: { editor: Editor }) {
  // Re-render on every transaction so the buttons reflect the selection.
  const active = useEditorState({
    editor,
    selector: ({ editor }) => ({
      bold: editor.isActive("bold"),
      italic: editor.isActive("italic"),
      strike: editor.isActive("strike"),
      h1: editor.isActive("heading", { level: 1 }),
      h2: editor.isActive("heading", { level: 2 }),
      h3: editor.isActive("heading", { level: 3 }),
      bullet: editor.isActive("bulletList"),
      ordered: editor.isActive("orderedList"),
      code: editor.isActive("codeBlock"),
      quote: editor.isActive("blockquote"),
    }),
  });
  const c = editor.chain().focus();

  const btn = (label: string, on: boolean, run: () => void, title: string, cls = "") => (
    <button
      type="button"
      onMouseDown={(e) => { e.preventDefault(); run(); }}
      aria-pressed={on}
      title={title}
      className={`h-6 min-w-6 rounded px-1.5 text-[12px] ${cls} ${on ? "bg-accent-soft text-accent" : "text-ink-muted hover:bg-surface-sunken hover:text-ink"}`}
    >
      {label}
    </button>
  );

  return (
    <div className="mb-1 flex flex-wrap items-center gap-0.5 rounded-md border border-border bg-surface-raised p-0.5">
      {btn("B", active.bold, () => c.toggleBold().run(), "Bold (Ctrl+B)", "font-bold")}
      {btn("I", active.italic, () => c.toggleItalic().run(), "Italic (Ctrl+I)", "italic")}
      {btn("S", active.strike, () => c.toggleStrike().run(), "Strikethrough", "line-through")}
      <span className="mx-1 h-4 w-px bg-border" />
      {btn("H1", active.h1, () => c.toggleHeading({ level: 1 }).run(), "Heading 1")}
      {btn("H2", active.h2, () => c.toggleHeading({ level: 2 }).run(), "Heading 2")}
      {btn("H3", active.h3, () => c.toggleHeading({ level: 3 }).run(), "Heading 3")}
      <span className="mx-1 h-4 w-px bg-border" />
      {btn("•", active.bullet, () => c.toggleBulletList().run(), "Bullet list")}
      {btn("1.", active.ordered, () => c.toggleOrderedList().run(), "Numbered list")}
      {btn("‹›", active.code, () => c.toggleCodeBlock().run(), "Code block")}
      {btn("“", active.quote, () => c.toggleBlockquote().run(), "Quote")}
      {btn("—", false, () => c.setHorizontalRule().run(), "Divider")}
    </div>
  );
}
