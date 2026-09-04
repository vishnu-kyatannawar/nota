/**
 * One rule, in one place: never pull keyboard focus out of something the user
 * is already typing in. Programmatic focus is otherwise a race — a note that
 * finishes loading in the background will happily steal the caret out of the
 * rename box, and a re-rendered editor will steal it out of an item row.
 *
 * Item rows are the deliberate exception: moving between them (Enter, arrows)
 * is the whole point of the list, so they are tagged and stay stealable.
 */

/** Marks an input that item-list navigation is allowed to take focus from. */
export const ITEM_ROW_ATTR = "data-item-row";

function editable(el: Element): boolean {
  if (el.closest(`[contenteditable="true"], [contenteditable=""]`)) return true;
  if (el instanceof HTMLTextAreaElement) return true;
  return el instanceof HTMLInputElement && !["checkbox", "radio", "button", "submit"].includes(el.type);
}

/**
 * True when focus currently sits somewhere that must not be interrupted: the
 * notes editor, a rename box, a dialog field or the search input.
 */
export function stealsFocus(): boolean {
  const el = document.activeElement;
  if (!el || el === document.body || el === document.documentElement) return false;
  if (el.hasAttribute(ITEM_ROW_ATTR)) return false;
  return editable(el);
}
