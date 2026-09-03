import { useEffect, useRef, useState } from "react";
import type { LocalItem, ItemsAction } from "../lib/items";
import { isMultiLinePaste, splitPastedList } from "../lib/items";
import { partsOf } from "../lib/api";
import { CodeEditor } from "./CodeEditor";

type Props = {
  item: LocalItem;
  index: number;
  focused: boolean;
  caret: "end" | number;
  dispatch: (a: ItemsAction) => void;
  dark: boolean;
};

/**
 * One action item. The text is a plain input: the stored format keeps #labels
 * inline as text, so an input round-trips exactly what the file holds. All
 * keyboard behaviour dispatches into the local reducer — nothing here waits on
 * the backend, which is what makes Enter and the arrows feel instant.
 */
export function ItemRow({ item, index, focused, caret, dispatch, dark }: Props) {
  const input = useRef<HTMLInputElement | null>(null);
  const [showBody, setShowBody] = useState(item.body.length > 0);

  useEffect(() => {
    if (!focused) return;
    const el = input.current;
    if (!el) return;
    el.focus();
    const pos = caret === "end" ? el.value.length : Math.min(caret, el.value.length);
    el.setSelectionRange(pos, pos);
  }, [focused, caret]);

  const onKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    const mod = e.ctrlKey || e.metaKey;
    switch (e.key) {
      case "Enter":
        e.preventDefault();
        if (mod) dispatch({ type: "toggle", index });
        else dispatch({ type: "insertAfter", index });
        return;
      case "ArrowUp":
        e.preventDefault();
        dispatch({ type: "move", index, delta: -1 });
        return;
      case "ArrowDown":
        e.preventDefault();
        dispatch({ type: "move", index, delta: 1 });
        return;
      case "Tab":
        e.preventDefault();
        dispatch({ type: "indent", index, delta: e.shiftKey ? -1 : 1 });
        return;
      case "Backspace":
        if (item.text === "" && item.body.length === 0) {
          e.preventDefault();
          dispatch({ type: "remove", index });
        }
        return;
      case "Escape":
        input.current?.blur();
        return;
      case "n":
      case "N":
        if (mod && e.shiftKey) {
          e.preventDefault();
          setShowBody(true);
        }
        return;
      default:
    }
  };

  const onPaste = (e: React.ClipboardEvent<HTMLInputElement>) => {
    const text = e.clipboardData.getData("text/plain");
    if (!isMultiLinePaste(text)) return;
    e.preventDefault();
    dispatch({ type: "insertMany", index, lines: splitPastedList(text) });
  };

  const { labels } = partsOf(item.text);

  return (
    <div className="group rounded-md px-1.5 py-1 hover:bg-surface-raised" style={{ marginLeft: item.depth * 22 }}>
      <div className="flex items-start gap-2.5">
        <button
          type="button"
          role="checkbox"
          aria-checked={item.done}
          aria-label={item.done ? "Mark as not done" : "Mark as done"}
          tabIndex={-1}
          onClick={() => dispatch({ type: "toggle", index })}
          className={`mt-[3px] flex h-4 w-4 shrink-0 items-center justify-center rounded border transition ${
            item.done ? "border-accent bg-accent text-accent-ink" : "border-border-strong bg-surface-raised hover:border-accent"
          }`}
        >
          {item.done && (
            <svg viewBox="0 0 16 16" width="11" height="11" aria-hidden="true">
              <path d="M3 8.5l3 3 7-7" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" />
            </svg>
          )}
        </button>

        <input
          ref={input}
          value={item.text}
          onChange={(e) => dispatch({ type: "setText", index, text: e.target.value })}
          onFocus={() => {
            if (!focused) dispatch({ type: "focus", index, caret: input.current?.selectionStart ?? "end" });
          }}
          onKeyDown={onKeyDown}
          onPaste={onPaste}
          placeholder={index === 0 && item.text === "" ? "Type an item…" : ""}
          aria-label="Item text"
          className={`min-w-0 flex-1 bg-transparent py-0.5 text-[14px] outline-none placeholder:text-ink-faint ${
            item.done ? "text-ink-faint line-through" : "text-ink"
          }`}
        />

        <div className="flex shrink-0 items-center gap-1.5 pt-[3px] text-[11px]">
          {labels.map((l) => (
            <span key={l} className="rounded-full bg-accent-soft px-1.5 py-px text-accent">#{l}</span>
          ))}
          {(item.carried ?? 0) > 0 && (
            <span className="rounded-full bg-surface-sunken px-1.5 py-px text-ink-muted" title={`First added on ${item.from}`}>
              {item.carried}d
            </span>
          )}
          <span className="font-mono text-ink-faint">
            {item.createdAt && item.createdAt}
            {item.doneAt && ` · ✓ ${item.doneAt}`}
          </span>
          <button
            type="button"
            tabIndex={-1}
            onClick={() => setShowBody((v) => !v)}
            aria-label={showBody ? "Hide notes" : "Add notes"}
            title="Notes for this item (Ctrl+Shift+N)"
            className={`rounded px-1 text-ink-faint hover:text-ink ${item.body.length > 0 || showBody ? "" : "opacity-0 group-hover:opacity-100"}`}
          >
            {item.body.length > 0 || showBody ? "▾" : "+"}
          </button>
        </div>
      </div>

      {showBody && (
        <div className="ml-[26px] mt-1.5">
          <CodeEditor
            value={item.body.join("\n")}
            minHeight="4rem"
            dark={dark}
            onChange={(v) => dispatch({ type: "setBody", index, body: v === "" ? [] : v.split("\n") })}
          />
        </div>
      )}
    </div>
  );
}
