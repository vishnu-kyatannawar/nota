import { useEffect, useRef, useState } from "react";
import type { LocalItem, ItemsAction } from "../lib/items";
import { enterSplit, headingShortcut, isMultiLinePaste, splitPastedList } from "../lib/items";
import { addLabel, joinLabels, labelAtCaret, removeLabel, splitLabels } from "../lib/labels";
import { ITEM_ROW_ATTR, stealsFocus } from "../lib/focus";
import { CodeEditor } from "./CodeEditor";
import { ContextMenu, type MenuItem } from "./ContextMenu";
import { Icon } from "./Icon";

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
  onError?: (message: string) => void;
};

/**
 * One row: an action item, or a group heading between items. Item text is a
 * plain input with #labels as chips beside it; the file keeps its trailing
 * #label format. The actions a person reaches for constantly — today, done,
 * delete — are one click on hover or one key, with ⋯ for the rest.
 */
export function ItemRow(props: Props) {
  return props.item.kind === "heading" ? <HeadingRow {...props} /> : <TaskRow {...props} />;
}

function useRowFocus<T extends HTMLInputElement | HTMLTextAreaElement>(focused: boolean, caret: "end" | number, grow?: string) {
  const input = useRef<T | null>(null);
  // A long item wraps, and the field grows to fit it rather than scrolling a
  // single line that hid everything past the first. Sized here because this is
  // where the element is owned.
  //
  // Measuring once is not enough. The height depends on where the text wraps,
  // and that changes after the first paint: the bundled faces arrive later and
  // reflow it, and the column is a different width whenever the window is
  // resized, the sidebar moves, or items and notes are put side by side.
  useEffect(() => {
    if (grow === undefined) return;
    const el = input.current;
    if (!el) return;
    const fit = () => {
      el.style.height = "auto";
      el.style.height = `${el.scrollHeight}px`;
    };
    fit();

    // Watch the parent, not the field: resizing the field changes its own
    // height, which would feed straight back into the observer.
    const parent = el.parentElement;
    const observer = parent && typeof ResizeObserver !== "undefined" ? new ResizeObserver(fit) : null;
    if (parent && observer) observer.observe(parent);

    let live = true;
    const fonts: FontFaceSet | undefined = document.fonts;
    if (fonts) void fonts.ready.then(() => { if (live) fit(); });

    return () => {
      live = false;
      observer?.disconnect();
    };
  }, [grow]);
  useEffect(() => {
    if (!focused) return;
    const el = input.current;
    if (!el) return;
    // Moving between rows is the point of the list, so a sibling row is fair
    // game — but a note finishing its load in the background must not pull the
    // caret out of the rename box or the notes editor.
    if (document.activeElement !== el && stealsFocus()) return;
    el.focus();
    const pos = caret === "end" ? el.value.length : Math.min(caret, el.value.length);
    el.setSelectionRange(pos, pos);
  }, [focused, caret]);
  return input;
}

function HeadingRow({ item, index, focused, caret, dispatch }: Props) {
  const input = useRowFocus<HTMLInputElement>(focused, caret);
  const [menu, setMenu] = useState<{ x: number; y: number } | null>(null);

  const onKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    const mod = e.ctrlKey || e.metaKey;
    switch (e.key) {
      case "Enter": e.preventDefault(); dispatch({ type: "insertAfter", index }); return;
      case "ArrowUp": e.preventDefault(); dispatch({ type: "move", index, delta: -1 }); return;
      case "ArrowDown": e.preventDefault(); dispatch({ type: "move", index, delta: 1 }); return;
      case "Backspace":
        if (mod || item.text === "") { e.preventDefault(); dispatch({ type: "remove", index }); }
        return;
      case "Delete": if (mod) { e.preventDefault(); dispatch({ type: "remove", index }); } return;
      case "Escape": input.current?.blur(); return;
      default:
    }
  };

  const menuItems: MenuItem[] = [
    { label: "Turn into item", onSelect: () => dispatch({ type: "setKind", index, kind: "" }) },
    { label: "Delete heading", onSelect: () => dispatch({ type: "remove", index }), danger: true },
  ];

  return (
    <div className={`group flex items-center gap-2 rounded-md px-1.5 pb-1 pt-3 ${index === 0 ? "pt-1" : ""}`}>
      <Icon name="heading" size={14} className="shrink-0 text-ink-muted" />
      <input
        ref={input}
        {...{ [ITEM_ROW_ATTR]: "" }}
        value={item.text}
        onChange={(e) => dispatch({ type: "setText", index, text: e.target.value })}
        onFocus={() => { if (!focused) dispatch({ type: "focus", index, caret: input.current?.selectionStart ?? "end" }); }}
        onKeyDown={onKeyDown}
        placeholder="Heading"
        aria-label="Heading"
        className="min-w-0 flex-1 bg-transparent py-0.5 text-[13px] font-semibold uppercase tracking-[0.08em] text-ink outline-none placeholder:text-ink-faint"
      />
      <RowButton label="Delete heading" danger onClick={() => dispatch({ type: "remove", index })}><Icon name="trash" /></RowButton>
      <RowButton label="More" onClick={(e) => setMenu({ x: e.clientX, y: e.clientY })}><Icon name="more" /></RowButton>
      {menu && <ContextMenu x={menu.x} y={menu.y} items={menuItems} onClose={() => setMenu(null)} />}
    </div>
  );
}

function TaskRow({ item, index, focused, caret, dispatch, dark, allLabels, onMoveToToday, onError }: Props) {
  const { plain, labels } = splitLabels(item.text);

  // The field edits `draft`; the saved text is recomposed from it and the chips.
  const [draft, setDraft] = useState(plain);
  const [lastPlain, setLastPlain] = useState(plain);
  if (plain !== lastPlain) {
    setLastPlain(plain);
    setDraft(plain);
  }

  const input = useRowFocus<HTMLTextAreaElement>(focused, caret, draft);

  const [showBody, setShowBody] = useState(item.body.length > 0);
  const [menu, setMenu] = useState<{ x: number; y: number } | null>(null);
  const [suggest, setSuggest] = useState<{ start: number; query: string; pick: number } | null>(null);

  const commitText = (text: string) => dispatch({ type: "setText", index, text });

  const setPlain = (value: string, caretPos: number) => {
    // "## " at the start of an empty row makes it a heading, markdown-style.
    if (item.text === "" && labels.length === 0) {
      const h = headingShortcut(value);
      if (h) {
        dispatch({ type: "setKind", index, kind: "heading" });
        commitText(h.rest);
        return;
      }
    }
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

  const toggleBody = () => {
    if (showBody) {
      dispatch({ type: "setBody", index, body: [] });
      setShowBody(false);
    } else {
      setShowBody(true);
    }
  };

  const onKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
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
        // The caret decides: at the start the row moves down, in the middle it
        // splits there, at the end this is just a new row below.
        else dispatch(enterSplit(index, draft, input.current?.selectionStart ?? draft.length, labels));
        return;
      case "ArrowUp": e.preventDefault(); dispatch({ type: "move", index, delta: -1 }); return;
      case "ArrowDown": e.preventDefault(); dispatch({ type: "move", index, delta: 1 }); return;
      case "Tab": e.preventDefault(); dispatch({ type: "indent", index, delta: e.shiftKey ? -1 : 1 }); return;
      case "Backspace": {
        const el = input.current;
        const atStart = el ? el.selectionStart === 0 && el.selectionEnd === 0 : false;
        if (mod) { e.preventDefault(); dispatch({ type: "remove", index }); }
        else if (draft === "" && labels.length === 0 && item.body.length === 0) { e.preventDefault(); dispatch({ type: "remove", index }); }
        else if (draft === "" && labels.length > 0) { e.preventDefault(); commitText(removeLabel(item.text, labels[labels.length - 1])); }
        // At the very start, fold this row into the one above — the reverse of
        // the split Enter does. The first row has nothing above it.
        else if (atStart && index > 0) { e.preventDefault(); dispatch({ type: "join", index }); }
        return;
      }
      case "Delete": if (mod) { e.preventDefault(); dispatch({ type: "remove", index }); } return;
      case "Escape": input.current?.blur(); return;
      case "n": case "N": if (mod && e.shiftKey) { e.preventDefault(); toggleBody(); } return;
      case "m": case "M": if (mod && e.shiftKey && onMoveToToday) { e.preventDefault(); onMoveToToday(); } return;
      default:
    }
  };

  const onPaste = (e: React.ClipboardEvent<HTMLTextAreaElement>) => {
    const text = e.clipboardData.getData("text/plain");
    if (!isMultiLinePaste(text)) return;
    e.preventDefault();
    dispatch({ type: "insertMany", index, lines: splitPastedList(text) });
  };

  const menuItems: MenuItem[] = [
    { label: showBody ? "Remove notes" : "Add notes", onSelect: toggleBody },
    { label: "Turn into heading", onSelect: () => dispatch({ type: "setKind", index, kind: "heading" }) },
    ...(onMoveToToday ? [{ label: "Move to today's workplan", onSelect: onMoveToToday }] : []),
    { label: "Delete item", onSelect: () => dispatch({ type: "remove", index }), danger: true },
  ];

  return (
    <div className="group relative rounded-md px-1.5 py-1 hover:bg-surface-raised focus-within:bg-surface-raised" style={{ marginLeft: item.depth * 22 }}>
      <div className="flex items-start gap-2.5">
        <button
          type="button"
          role="checkbox"
          aria-checked={item.done}
          aria-label={item.done ? "Mark as not done" : "Mark as done"}
          tabIndex={-1}
          onClick={() => dispatch({ type: "toggle", index })}
          className={`mt-[2px] flex h-[17px] w-[17px] shrink-0 items-center justify-center rounded-[4px] border-[1.5px] transition ${
            item.done ? "border-accent bg-accent text-accent-ink" : "border-border-strong bg-surface-raised hover:border-accent"
          }`}
        >
          {item.done && <Icon name="check" size={12} className="[&>path]:stroke-[3]" />}
        </button>

        <div className="relative min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-1.5">
            <textarea
              ref={input}
              {...{ [ITEM_ROW_ATTR]: "" }}
              rows={1}
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
              placeholder={index === 0 && item.text === "" ? "Type an item, or ## for a heading…" : ""}
              aria-label="Item text"
              className={`min-w-[8rem] flex-1 resize-none overflow-hidden break-words bg-transparent py-0.5 leading-[1.5] outline-none placeholder:text-ink-faint ${
                item.done ? "text-ink-faint line-through" : "text-ink"
              }`}
            />
            {labels.map((l) => (
              <span key={l} className="inline-flex items-center gap-0.5 rounded-full bg-accent-soft pl-1.5 pr-0.5 text-[12px] leading-5 text-accent">
                #{l}
                <button type="button" tabIndex={-1} aria-label={`Remove label ${l}`} onClick={() => commitText(removeLabel(item.text, l))} className="rounded-full px-1 hover:bg-accent hover:text-accent-ink">×</button>
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
                  #{s}{!allLabels.includes(s) && <span className="ml-1 text-ink-muted">new</span>}
                </button>
              ))}
            </div>
          )}
        </div>

        <div className="flex shrink-0 items-center gap-0.5 pt-[1px]">
          {(item.carried ?? 0) > 0 && (
            <span className="mr-1 rounded-full bg-surface-sunken px-1.5 py-px text-[11px] text-ink-muted" title={`First added on ${item.from}`}>{item.carried}d</span>
          )}
          <span className="mr-1 font-mono text-[11px] text-ink-muted">{item.createdAt}{item.doneAt && ` · ✓ ${item.doneAt}`}</span>
          {onMoveToToday && (
            <RowButton label="Move to today's workplan (Ctrl+Shift+M)" onClick={onMoveToToday}><Icon name="today" /></RowButton>
          )}
          <RowButton label={item.done ? "Reopen (Ctrl+Enter)" : "Mark done (Ctrl+Enter)"} onClick={() => dispatch({ type: "toggle", index })}>
            <Icon name={item.done ? "undo" : "check"} />
          </RowButton>
          <RowButton label="Delete item (Ctrl+Backspace)" danger onClick={() => dispatch({ type: "remove", index })}><Icon name="trash" /></RowButton>
          <RowButton label="More" onClick={(e) => setMenu({ x: e.clientX, y: e.clientY })}><Icon name="more" /></RowButton>
        </div>
      </div>

      {showBody && (
        <div className="ml-[27px] mt-1.5">
          <div className="mb-1 flex items-center text-[11px] font-semibold uppercase tracking-[0.1em] text-ink-muted">
            Notes
            <button type="button" tabIndex={-1} onClick={toggleBody} aria-label="Remove notes" className="ml-auto rounded px-1 text-[13px] normal-case tracking-normal hover:text-danger">×</button>
          </div>
          <CodeEditor
            value={item.body.join("\n")}
            minHeight="4rem"
            dark={dark}
            onChange={(v) => dispatch({ type: "setBody", index, body: v === "" ? [] : v.split("\n") })}
            onEmptyBackspace={toggleBody}
            onError={onError}
          />
        </div>
      )}

      {menu && <ContextMenu x={menu.x} y={menu.y} items={menuItems} onClose={() => setMenu(null)} />}
    </div>
  );
}

function RowButton({ label, danger, onClick, children }: { label: string; danger?: boolean; onClick: (e: React.MouseEvent<HTMLButtonElement>) => void; children: React.ReactNode }) {
  return (
    <button
      type="button"
      tabIndex={-1}
      title={label}
      aria-label={label}
      onClick={onClick}
      className={`flex h-6 w-6 items-center justify-center rounded text-ink-muted opacity-0 transition group-hover:opacity-100 group-focus-within:opacity-100 focus:opacity-100 ${
        danger ? "hover:bg-danger/10 hover:text-danger" : "hover:bg-accent-soft hover:text-accent"
      }`}
    >
      {children}
    </button>
  );
}
