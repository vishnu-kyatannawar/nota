import { useEffect, useRef, useState } from "react";
import type { LocalItem, ItemsAction } from "../lib/items";
import { isMultiLinePaste, splitPastedList } from "../lib/items";
import { addLabel, joinLabels, labelAtCaret, removeLabel, splitLabels } from "../lib/labels";
import { CodeEditor } from "./CodeEditor";
import { ContextMenu, type MenuItem } from "./ContextMenu";

type Props = {
  item: LocalItem;
  index: number;
  focused: boolean;
  caret: "end" | number;
  dispatch: (a: ItemsAction) => void;
  dark: boolean;
  allLabels: string[];
  /** Present when this note is not today's workplan. */
  onMoveToToday?: () => void;
};

/**
 * One action item. The input holds the item's words; its #labels render as
 * chips beside it with an × each, and typing "#" offers existing labels. The
 * file keeps the same trailing-#label format it always had — only the way it
 * is shown changed.
 */
export function ItemRow({ item, index, focused, caret, dispatch, dark, allLabels, onMoveToToday }: Props) {
  const input = useRef<HTMLInputElement | null>(null);
  const { plain, labels } = splitLabels(item.text);

  // The input edits `draft`; the saved text is recomposed from it and the chips.
  const [draft, setDraft] = useState(plain);
  const [lastPlain, setLastPlain] = useState(plain);
  if (plain !== lastPlain) {
    setLastPlain(plain);
    setDraft(plain);
  }

  const [showBody, setShowBody] = useState(item.body.length > 0);
  const [menu, setMenu] = useState<{ x: number; y: number } | null>(null);
  const [suggest, setSuggest] = useState<{ start: number; query: string; pick: number } | null>(null);

  useEffect(() => {
    if (!focused) return;
    const el = input.current;
    if (!el) return;
    el.focus();
    const pos = caret === "end" ? el.value.length : Math.min(caret, el.value.length);
    el.setSelectionRange(pos, pos);
  }, [focused, caret]);

  const commitText = (text: string) => dispatch({ type: "setText", index, text });

  const setPlain = (value: string, caretPos: number) => {
    // A finished "#label " inside the words becomes a chip straight away.
    if (/(?:^|\s)#[\p{L}\p{N}_/-]+\s$/u.test(value)) {
      const typed = splitLabels(value);
      setDraft(typed.plain);
      commitText(joinLabels(typed.plain, [...labels, ...typed.labels]));
      setSuggest(null);
      return;
    }
    setDraft(value);
    commitText(joinLabels(value, labels));
    const at = labelAtCaret(value, caretPos);
    setSuggest(at ? { ...at, pick: 0 } : null);
  };

  const suggestions = suggest
    ? [
        ...allLabels.filter((l) => !labels.includes(l) && l.toLowerCase().startsWith(suggest.query.toLowerCase())),
        ...(suggest.query && !allLabels.includes(suggest.query) ? [suggest.query] : []),
      ].slice(0, 8)
    : [];

  const pickSuggestion = (name: string) => {
    if (!suggest) return;
    const withoutToken = (draft.slice(0, suggest.start) + draft.slice(suggest.start + 1 + suggest.query.length)).replace(/\s+$/, "");
    setDraft(withoutToken);
    commitText(addLabel(joinLabels(withoutToken, labels), name));
    setSuggest(null);
  };

  const onKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    const mod = e.ctrlKey || e.metaKey;

    if (suggest && suggestions.length > 0) {
      if (e.key === "ArrowDown") { e.preventDefault(); setSuggest({ ...suggest, pick: (suggest.pick + 1) % suggestions.length }); return; }
      if (e.key === "ArrowUp") { e.preventDefault(); setSuggest({ ...suggest, pick: (suggest.pick - 1 + suggestions.length) % suggestions.length }); return; }
      if (e.key === "Enter" || e.key === "Tab") { e.preventDefault(); pickSuggestion(suggestions[suggest.pick]); return; }
      if (e.key === "Escape") { e.preventDefault(); setSuggest(null); return; }
    }

    switch (e.key) {
      case "Enter":
        e.preventDefault();
        if (mod) dispatch({ type: "toggle", index });
        else dispatch({ type: "insertAfter", index });
        return;
      case "ArrowUp": e.preventDefault(); dispatch({ type: "move", index, delta: -1 }); return;
      case "ArrowDown": e.preventDefault(); dispatch({ type: "move", index, delta: 1 }); return;
      case "Tab": e.preventDefault(); dispatch({ type: "indent", index, delta: e.shiftKey ? -1 : 1 }); return;
      case "Backspace":
        if (draft === "" && labels.length === 0 && item.body.length === 0) { e.preventDefault(); dispatch({ type: "remove", index }); }
        else if (draft === "" && labels.length > 0) { e.preventDefault(); commitText(removeLabel(item.text, labels[labels.length - 1])); }
        return;
      case "Escape": input.current?.blur(); return;
      case "n": case "N":
        if (mod && e.shiftKey) { e.preventDefault(); toggleBody(); }
        return;
      case "m": case "M":
        if (mod && e.shiftKey && onMoveToToday) { e.preventDefault(); onMoveToToday(); }
        return;
      default:
    }
  };

  const toggleBody = () => {
    if (showBody) {
      dispatch({ type: "setBody", index, body: [] });
      setShowBody(false);
    } else {
      setShowBody(true);
    }
  };

  const onPaste = (e: React.ClipboardEvent<HTMLInputElement>) => {
    const text = e.clipboardData.getData("text/plain");
    if (!isMultiLinePaste(text)) return;
    e.preventDefault();
    dispatch({ type: "insertMany", index, lines: splitPastedList(text) });
  };

  const menuItems: MenuItem[] = [
    ...(onMoveToToday ? [{ label: "Move to today's workplan", onSelect: onMoveToToday }] : []),
    { label: showBody ? "Remove notes" : "Add notes", onSelect: toggleBody },
    { label: item.done ? "Mark not done" : "Mark done", onSelect: () => dispatch({ type: "toggle", index }) },
    { label: "Delete item", onSelect: () => dispatch({ type: "remove", index }), danger: true },
  ];

  return (
    <div className="group relative rounded-md px-1.5 py-1 hover:bg-surface-raised" style={{ marginLeft: item.depth * 22 }}>
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

        <div className="relative min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-1.5">
            <input
              ref={input}
              value={draft}
              onChange={(e) => setPlain(e.target.value, e.target.selectionStart ?? e.target.value.length)}
              onFocus={() => { if (!focused) dispatch({ type: "focus", index, caret: input.current?.selectionStart ?? "end" }); }}
              onBlur={() => {
                setSuggest(null);
                const typed = splitLabels(draft);
                if (typed.labels.length) { setDraft(typed.plain); commitText(joinLabels(typed.plain, [...labels, ...typed.labels])); }
              }}
              onKeyDown={onKeyDown}
              onPaste={onPaste}
              placeholder={index === 0 && item.text === "" ? "Type an item…" : ""}
              aria-label="Item text"
              className={`min-w-[8rem] flex-1 bg-transparent py-0.5 text-[14px] outline-none placeholder:text-ink-faint ${
                item.done ? "text-ink-faint line-through" : "text-ink"
              }`}
            />
            {labels.map((l) => (
              <span key={l} className="inline-flex items-center gap-0.5 rounded-full bg-accent-soft pl-1.5 pr-0.5 text-[11px] leading-5 text-accent">
                #{l}
                <button
                  type="button"
                  tabIndex={-1}
                  aria-label={`Remove label ${l}`}
                  onClick={() => commitText(removeLabel(item.text, l))}
                  className="rounded-full px-1 hover:bg-accent hover:text-accent-ink"
                >
                  ×
                </button>
              </span>
            ))}
          </div>

          {suggest && suggestions.length > 0 && (
            <div role="listbox" className="absolute left-0 top-full z-20 mt-1 min-w-40 rounded-md border border-border bg-surface-raised py-1 text-[12px] shadow-lg">
              {suggestions.map((s, i) => (
                <button
                  key={s}
                  type="button"
                  role="option"
                  aria-selected={i === suggest.pick}
                  onMouseDown={(e) => { e.preventDefault(); pickSuggestion(s); }}
                  className={`block w-full px-2.5 py-1 text-left ${i === suggest.pick ? "bg-accent-soft text-accent" : "hover:bg-surface-sunken"}`}
                >
                  #{s}{!allLabels.includes(s) && <span className="ml-1 text-ink-faint">new</span>}
                </button>
              ))}
            </div>
          )}
        </div>

        <div className="flex shrink-0 items-center gap-1.5 pt-[3px] text-[11px]">
          {(item.carried ?? 0) > 0 && (
            <span className="rounded-full bg-surface-sunken px-1.5 py-px text-ink-muted" title={`First added on ${item.from}`}>{item.carried}d</span>
          )}
          <span className="font-mono text-ink-faint">{item.createdAt}{item.doneAt && ` · ✓ ${item.doneAt}`}</span>
          <button
            type="button"
            tabIndex={-1}
            aria-label="Item actions"
            onClick={(e) => setMenu({ x: e.clientX, y: e.clientY })}
            className="rounded px-1 text-ink-faint opacity-0 hover:text-ink group-hover:opacity-100 focus:opacity-100"
          >
            ⋯
          </button>
        </div>
      </div>

      {showBody && (
        <div className="ml-[26px] mt-1.5">
          <div className="mb-1 flex items-center text-[10px] font-semibold uppercase tracking-[0.12em] text-ink-faint">
            Notes
            <button type="button" tabIndex={-1} onClick={toggleBody} aria-label="Remove notes" className="ml-auto rounded px-1 text-[12px] normal-case tracking-normal hover:text-danger">×</button>
          </div>
          <CodeEditor
            value={item.body.join("\n")}
            minHeight="4rem"
            dark={dark}
            onChange={(v) => dispatch({ type: "setBody", index, body: v === "" ? [] : v.split("\n") })}
          />
        </div>
      )}

      {menu && <ContextMenu x={menu.x} y={menu.y} items={menuItems} onClose={() => setMenu(null)} />}
    </div>
  );
}
