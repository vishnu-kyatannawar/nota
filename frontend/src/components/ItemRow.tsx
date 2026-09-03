import { useEffect, useRef, useState } from "react";
import type { NoteItem } from "../lib/api";
import { partsOf } from "../lib/api";
import { CodeEditor } from "./CodeEditor";

type Props = {
  item: NoteItem;
  onToggle: () => void;
  onText: (text: string) => void;
  onEnter: () => void;
  onIndent: (delta: number) => void;
  onRemove: () => void;
  onLogTime: () => void;
  onBody: (body: string[]) => void;
  focusOnMount?: boolean;
};

/**
 * One action item. The text is an ordinary input rather than a rich-text field:
 * the stored format keeps #labels and [hh:mm] inline as plain text, so a
 * controlled input round-trips exactly what the file holds, with no conversion
 * layer to lose anything.
 */
export function ItemRow({
  item,
  onToggle,
  onText,
  onEnter,
  onIndent,
  onRemove,
  onLogTime,
  onBody,
  focusOnMount,
}: Props) {
  const [text, setText] = useState(item.text);
  const [showBody, setShowBody] = useState(item.body.length > 0);
  const input = useRef<HTMLInputElement | null>(null);

  // When the saved text changes underneath us — another edit, a reload, a file
  // changed on disk — adopt it. Adjusting state during render is React's own
  // guidance for this, and avoids the extra render an effect would cost.
  const [lastSaved, setLastSaved] = useState(item.text);
  if (item.text !== lastSaved) {
    setLastSaved(item.text);
    setText(item.text);
  }

  useEffect(() => {
    if (focusOnMount) input.current?.focus();
  }, [focusOnMount]);

  const commit = () => {
    if (text !== item.text) onText(text);
  };

  const onKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter" && (e.ctrlKey || e.metaKey)) {
      e.preventDefault();
      onToggle();
      return;
    }
    if (e.key === "Enter") {
      e.preventDefault();
      commit();
      onEnter();
      return;
    }
    if (e.key === "Tab") {
      e.preventDefault();
      commit();
      onIndent(e.shiftKey ? -1 : 1);
      return;
    }
    if (e.key === "t" && (e.ctrlKey || e.metaKey)) {
      e.preventDefault();
      onLogTime();
      return;
    }
    if (e.key === "n" && (e.ctrlKey || e.metaKey) && e.shiftKey) {
      e.preventDefault();
      setShowBody(true);
      return;
    }
    if (e.key === "Backspace" && text === "") {
      e.preventDefault();
      onRemove();
    }
  };

  const { labels, time } = partsOf(item.text);

  return (
    <div className="group rounded px-1 py-0.5 hover:bg-white/5" style={{ marginLeft: item.depth * 20 }}>
      <div className="flex items-start gap-2">
        <button
          type="button"
          role="checkbox"
          aria-checked={item.done}
          aria-label={item.done ? "Mark as not done" : "Mark as done"}
          onClick={onToggle}
          className={`mt-1.5 h-[15px] w-[15px] shrink-0 rounded-sm border ${
            item.done ? "border-accent bg-accent" : "border-white/30"
          }`}
        />

        <input
          ref={input}
          value={text}
          onChange={(e) => setText(e.target.value)}
          onBlur={commit}
          onKeyDown={onKeyDown}
          aria-label="Item text"
          className={`min-w-0 flex-1 bg-transparent py-0.5 text-sm outline-none ${
            item.done ? "text-ink-muted line-through" : "text-ink"
          }`}
        />

        <div className="flex shrink-0 items-center gap-2 pt-1 text-[11px]">
          {time && <span className="font-mono text-amber-400">[{time}]</span>}
          {labels.map((l) => (
            <span key={l} className="rounded-full bg-accent/15 px-1.5 text-accent">
              #{l}
            </span>
          ))}
          {item.carried > 0 && (
            <span className="rounded-full bg-amber-500/15 px-1.5 text-amber-400">
              carried {item.carried}d
            </span>
          )}
          <span className="font-mono text-ink-muted/70">
            {item.createdAt && `added ${item.createdAt}`}
            {item.from && ` on ${item.from.slice(5)}`}
            {item.doneAt && ` · done ${item.doneAt}`}
          </span>
          <button
            type="button"
            onClick={() => setShowBody((v) => !v)}
            aria-label={showBody ? "Hide item notes" : "Add item notes"}
            className="opacity-0 transition group-hover:opacity-60 hover:!opacity-100"
          >
            {item.body.length > 0 || showBody ? "▾" : "+"}
          </button>
        </div>
      </div>

      {showBody && (
        <div className="ml-6 mt-1.5 border-l-2 border-white/10 pl-3">
          <CodeEditor
            value={item.body.join("\n")}
            minHeight="4rem"
            onChange={(v) => onBody(v === "" ? [] : v.split("\n"))}
          />
        </div>
      )}
    </div>
  );
}
