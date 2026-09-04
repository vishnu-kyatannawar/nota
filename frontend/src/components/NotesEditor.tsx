import { useEffect, useRef } from "react";
import { EditorContent, useEditor, useEditorState, type Editor } from "@tiptap/react";
import StarterKit from "@tiptap/starter-kit";
import { Markdown } from "@tiptap/markdown";
import CodeBlockLowlight from "@tiptap/extension-code-block-lowlight";
import Placeholder from "@tiptap/extension-placeholder";
import Image from "@tiptap/extension-image";
import { imagesIn, storeImages } from "../lib/paste";
import { common, createLowlight } from "lowlight";

const lowlight = createLowlight(common);

type Props = {
  value: string;
  onChange: (markdown: string) => void;
  onBlur?: () => void;
  onError?: (message: string) => void;
  placeholder?: string;
};

/**
 * The free-form notes area under a note's items. A WYSIWYG editor whose
 * document is markdown on disk: Tiptap reads the note's body and writes it
 * back through @tiptap/markdown. Only this section is ever re-serialised —
 * the item lines above it are never touched by this editor.
 */
export function NotesEditor({ value, onChange, onBlur, onError, placeholder = "Notes…" }: Props) {
  const handlers = useRef({ onChange, onBlur, onError });
  useEffect(() => {
    handlers.current = { onChange, onBlur, onError };
  }, [onChange, onBlur, onError]);

  const editor = useEditor({
    extensions: [
      StarterKit.configure({ codeBlock: false }),
      CodeBlockLowlight.configure({ lowlight }),
      Markdown,
      Placeholder.configure({ placeholder }),
      // The src stays exactly what the markdown says — "attachments/x.png" —
      // and the app serves that path, so the same link works outside Nota too.
      Image.configure({ inline: false }),
    ],
    content: value,
    contentType: "markdown",
    onUpdate: ({ editor }) => handlers.current.onChange(editor.getMarkdown()),
    onBlur: () => handlers.current.onBlur?.(),
    editorProps: {
      handlePaste: (view, event) => {
        const files = imagesIn(event.clipboardData);
        if (files.length === 0) return false;
        // Go writes the bytes into the vault and gives back the path; until
        // that returns there is nothing to insert, so the paste is swallowed.
        void storeImages(files)
          .then((links) => {
            const { state } = view;
            view.dispatch(state.tr.replaceSelectionWith(
              state.schema.nodes.paragraph.create(null, state.schema.text(links.join(" "))),
            ));
            editorRef.current?.commands.setContent(editorRef.current.getMarkdown(), { contentType: "markdown" });
          })
          .catch((e) => handlers.current.onError?.(String(e)));
        return true;
      },
      handleDrop: (view, event) => {
        const files = imagesIn((event as DragEvent).dataTransfer);
        if (files.length === 0) return false;
        event.preventDefault();
        void storeImages(files)
          .then((links) => {
            const { state } = view;
            view.dispatch(state.tr.replaceSelectionWith(
              state.schema.nodes.paragraph.create(null, state.schema.text(links.join(" "))),
            ));
            editorRef.current?.commands.setContent(editorRef.current.getMarkdown(), { contentType: "markdown" });
          })
          .catch((e) => handlers.current.onError?.(String(e)));
        return true;
      },
    },
  });

  // The paste handlers run outside React and need the editor they belong to.
  const editorRef = useRef<typeof editor>(null);
  useEffect(() => {
    editorRef.current = editor;
  }, [editor]);

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
  const btn = (label: string, on: boolean, run: () => void, title: string, cls = "") => (
    <button
      type="button"
      onMouseDown={(e) => { e.preventDefault(); run(); }}
      aria-pressed={on}
      title={title}
      className={`h-7 min-w-7 rounded px-1.5 text-[13px] ${cls} ${on ? "bg-accent-soft text-accent" : "text-ink hover:bg-surface-sunken"}`}
    >
      {label}
    </button>
  );

  return (
    <div className="mb-1 flex flex-wrap items-center gap-0.5 rounded-md border border-border bg-surface-raised p-0.5">
      {btn("B", active.bold, () => editor.chain().focus().toggleBold().run(), "Bold (Ctrl+B)", "font-bold")}
      {btn("I", active.italic, () => editor.chain().focus().toggleItalic().run(), "Italic (Ctrl+I)", "italic")}
      {btn("S", active.strike, () => editor.chain().focus().toggleStrike().run(), "Strikethrough", "line-through")}
      <span className="mx-1 h-4 w-px bg-border" />
      {btn("H1", active.h1, () => editor.chain().focus().toggleHeading({ level: 1 }).run(), "Heading 1")}
      {btn("H2", active.h2, () => editor.chain().focus().toggleHeading({ level: 2 }).run(), "Heading 2")}
      {btn("H3", active.h3, () => editor.chain().focus().toggleHeading({ level: 3 }).run(), "Heading 3")}
      <span className="mx-1 h-4 w-px bg-border" />
      {btn("•", active.bullet, () => editor.chain().focus().toggleBulletList().run(), "Bullet list")}
      {btn("1.", active.ordered, () => editor.chain().focus().toggleOrderedList().run(), "Numbered list")}
      {btn("‹›", active.code, () => editor.chain().focus().toggleCodeBlock().run(), "Code block")}
      {btn("“", active.quote, () => editor.chain().focus().toggleBlockquote().run(), "Quote")}
      {btn("—", false, () => editor.chain().focus().setHorizontalRule().run(), "Divider")}
    </div>
  );
}
