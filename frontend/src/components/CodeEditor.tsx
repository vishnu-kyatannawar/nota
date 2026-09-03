import { useEffect, useRef } from "react";
import { EditorView, basicSetup } from "codemirror";
import { EditorState } from "@codemirror/state";
import { keymap } from "@codemirror/view";
import { markdown } from "@codemirror/lang-markdown";
import { languages } from "@codemirror/language-data";
import { oneDark } from "@codemirror/theme-one-dark";

type Props = {
  value: string;
  onChange: (value: string) => void;
  onExit?: () => void;
  minHeight?: string;
  autoFocus?: boolean;
};

/**
 * Markdown source editor, used for an item's body and for the whole-note raw
 * mode. CodeMirror is here for the same reason Obsidian and Joplin use it: this
 * is markdown source, with fenced code inside it that wants real highlighting.
 * `languages` gives the fences their own grammars, so a ```go block reads as Go.
 */
export function CodeEditor({ value, onChange, onExit, minHeight = "8rem", autoFocus }: Props) {
  const host = useRef<HTMLDivElement | null>(null);
  const view = useRef<EditorView | null>(null);
  // Held in a ref so changing the handler does not tear down and rebuild the
  // editor, which would lose the cursor position mid-typing. Assigned in an
  // effect rather than during render, which would be a side effect in render.
  const handlers = useRef({ onChange, onExit });
  useEffect(() => {
    handlers.current = { onChange, onExit };
  }, [onChange, onExit]);

  useEffect(() => {
    if (!host.current) return;

    const state = EditorState.create({
      doc: value,
      extensions: [
        basicSetup,
        markdown({ codeLanguages: languages }),
        oneDark,
        EditorView.lineWrapping,
        keymap.of([
          {
            key: "Mod-e",
            run: () => {
              handlers.current.onExit?.();
              return true;
            },
          },
        ]),
        EditorView.updateListener.of((update) => {
          if (update.docChanged) handlers.current.onChange(update.state.doc.toString());
        }),
        EditorView.theme({
          "&": { minHeight, fontSize: "13px", background: "transparent" },
          ".cm-scroller": { fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace" },
          "&.cm-focused": { outline: "none" },
        }),
      ],
    });

    const editor = new EditorView({ state, parent: host.current });
    view.current = editor;
    if (autoFocus) editor.focus();

    return () => {
      editor.destroy();
      view.current = null;
    };
    // Rebuilding on every keystroke would fight the editor's own state, so the
    // document is only seeded once and kept in sync by the effect below.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Adopt a value that changed outside this editor — switching notes, or a file
  // edited in another application — without disturbing local typing.
  useEffect(() => {
    const editor = view.current;
    if (!editor) return;
    const current = editor.state.doc.toString();
    if (current === value) return;
    editor.dispatch({ changes: { from: 0, to: current.length, insert: value } });
  }, [value]);

  return <div ref={host} className="overflow-hidden rounded border border-surface-border" />;
}
