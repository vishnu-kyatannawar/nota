import { useEffect, useRef } from "react";
import { EditorView, basicSetup } from "codemirror";
import { Compartment, EditorState, RangeSetBuilder, StateField } from "@codemirror/state";
import { Decoration, EditorView as View, WidgetType, keymap, type DecorationSet } from "@codemirror/view";
import { imagesIn, storeImages } from "../lib/paste";
import { markdown } from "@codemirror/lang-markdown";
import { languages } from "@codemirror/language-data";
import { oneDark } from "@codemirror/theme-one-dark";

type Props = {
  value: string;
  onChange: (value: string) => void;
  onExit?: () => void;
  /** Called when Backspace is pressed and there is nothing left to delete. */
  onEmptyBackspace?: () => void;
  onError?: (message: string) => void;
  minHeight?: string;
  autoFocus?: boolean;
  dark: boolean;
};

/** Shows the picture under a line that is nothing but an image link. */
class ImageWidget extends WidgetType {
  constructor(private readonly src: string, private readonly alt: string) {
    super();
  }
  eq(other: ImageWidget) {
    return other.src === this.src && other.alt === this.alt;
  }
  toDOM() {
    const wrap = document.createElement("div");
    wrap.className = "cm-nota-image";
    const img = document.createElement("img");
    img.src = this.src;
    img.alt = this.alt;
    wrap.appendChild(img);
    return wrap;
  }
}

const IMAGE_LINE = /^!\[([^\]]*)\]\(([^)\s]+)\)\s*$/;

// Markdown is the file; this only draws what the markdown already says.
//
// A StateField rather than a ViewPlugin: CodeMirror refuses block decorations
// from a plugin, and the refusal is an exception thrown on every update — which
// aborted the very insert that pasting an image was making.
const images = StateField.define<DecorationSet>({
  create: (state) => build(state),
  update: (deco, tr) => (tr.docChanged ? build(tr.state) : deco),
  provide: (f) => View.decorations.from(f),
});

function build(state: EditorState): DecorationSet {
  const b = new RangeSetBuilder<Decoration>();
  for (let i = 1; i <= state.doc.lines; i++) {
    const line = state.doc.line(i);
    const m = IMAGE_LINE.exec(line.text.trim());
    if (m) b.add(line.to, line.to, Decoration.widget({ widget: new ImageWidget(m[2], m[1]), side: 1, block: true }));
  }
  return b.finish();
}

const themeCompartment = new Compartment();

/**
 * Markdown source editor for an item's notes and for the whole-note raw mode.
 * `languages` gives fenced blocks their own grammars, so ```go reads as Go.
 * The colour theme lives in a Compartment so it can follow the app theme
 * without rebuilding the editor and losing the cursor.
 */
export function CodeEditor({ value, onChange, onExit, onEmptyBackspace, onError, minHeight = "8rem", autoFocus, dark }: Props) {
  const host = useRef<HTMLDivElement | null>(null);
  const view = useRef<EditorView | null>(null);
  const handlers = useRef({ onChange, onExit, onEmptyBackspace, onError });
  useEffect(() => {
    handlers.current = { onChange, onExit, onEmptyBackspace, onError };
  }, [onChange, onExit, onEmptyBackspace, onError]);

  useEffect(() => {
    if (!host.current) return;
    const state = EditorState.create({
      doc: value,
      extensions: [
        basicSetup,
        markdown({ codeLanguages: languages }),
        themeCompartment.of(dark ? oneDark : []),
        EditorView.lineWrapping,
        images,
        keymap.of([
          { key: "Mod-e", run: () => { handlers.current.onExit?.(); return true; } },
          // Backspace with nothing left takes the notes away, rather than
          // leaving an empty box open under the item.
          {
            key: "Backspace",
            run: (v) => {
              if (v.state.doc.length > 0 || !handlers.current.onEmptyBackspace) return false;
              handlers.current.onEmptyBackspace();
              return true;
            },
          },
        ]),
        View.domEventHandlers({
          paste: (event, v) => {
            const files = imagesIn(event.clipboardData);
            if (files.length === 0) return false;
            event.preventDefault();
            void storeImages(files)
              .then((links) => {
                const at = v.state.selection.main;
                const before = at.from > 0 && v.state.doc.sliceString(at.from - 1, at.from) !== "\n" ? "\n" : "";
                v.dispatch({ changes: { from: at.from, to: at.to, insert: `${before}${links.join("\n")}\n` } });
              })
              .catch((e) => handlers.current.onError?.(String(e)));
            return true;
          },
        }),
        EditorView.updateListener.of((u) => {
          if (u.docChanged) handlers.current.onChange(u.state.doc.toString());
        }),
        EditorView.theme({
          "&": { minHeight, fontSize: "13px" },
          ".cm-scroller": { fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace", lineHeight: "1.55" },
          ".cm-content": { padding: "8px 0" },
          ".cm-nota-image": { padding: "4px 0 8px" },
          ".cm-nota-image img": { maxWidth: "100%", maxHeight: "22rem", borderRadius: "6px", display: "block" },
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
