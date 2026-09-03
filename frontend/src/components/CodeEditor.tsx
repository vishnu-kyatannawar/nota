import { useEffect, useRef } from "react";
import { EditorView, basicSetup } from "codemirror";
import { Compartment, EditorState } from "@codemirror/state";
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
  dark: boolean;
};

const themeCompartment = new Compartment();

/**
 * Markdown source editor for an item's notes and for the whole-note raw mode.
 * `languages` gives fenced blocks their own grammars, so ```go reads as Go.
 * The colour theme lives in a Compartment so it can follow the app theme
 * without rebuilding the editor and losing the cursor.
 */
export function CodeEditor({ value, onChange, onExit, minHeight = "8rem", autoFocus, dark }: Props) {
  const host = useRef<HTMLDivElement | null>(null);
  const view = useRef<EditorView | null>(null);
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
        themeCompartment.of(dark ? oneDark : []),
        EditorView.lineWrapping,
        keymap.of([{ key: "Mod-e", run: () => { handlers.current.onExit?.(); return true; } }]),
        EditorView.updateListener.of((u) => {
          if (u.docChanged) handlers.current.onChange(u.state.doc.toString());
        }),
        EditorView.theme({
          "&": { minHeight, fontSize: "13px" },
          ".cm-scroller": { fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace", lineHeight: "1.55" },
          ".cm-content": { padding: "8px 0" },
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
    // The document is seeded once; later values are adopted by the effect below.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    view.current?.dispatch({ effects: themeCompartment.reconfigure(dark ? oneDark : []) });
  }, [dark]);

  useEffect(() => {
    const editor = view.current;
    if (!editor) return;
    const current = editor.state.doc.toString();
    if (current === value) return;
    editor.dispatch({ changes: { from: 0, to: current.length, insert: value } });
  }, [value]);

  return <div ref={host} className="overflow-hidden rounded-md border border-border" />;
}
