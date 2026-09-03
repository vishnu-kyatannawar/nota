/**
 * Labels are stored inside an item's text as "#label" tokens, kept at the end
 * of the line. These helpers split that text into the plain part and the
 * labels, and put it back together, so the UI can show chips and an input
 * while the file keeps exactly the same format it always had.
 */

const LABEL = /#([\p{L}\p{N}][\p{L}\p{N}_/-]*)/gu;

export function splitLabels(text: string): { plain: string; labels: string[] } {
  const labels: string[] = [];
  for (const m of text.matchAll(LABEL)) {
    if (!labels.includes(m[1])) labels.push(m[1]);
  }
  const plain = text.replace(LABEL, "").replace(/\s+/g, " ").trim();
  return { plain, labels };
}

/** Composes an item's text: plain words first, labels trailing and deduplicated. */
export function joinLabels(plain: string, labels: string[]): string {
  const unique = labels.map(cleanLabel).filter((l, i, a) => l && a.indexOf(l) === i);
  const head = plain.replace(/\s+/g, " ").trim();
  const tail = unique.map((l) => `#${l}`).join(" ");
  return [head, tail].filter(Boolean).join(" ");
}

/** Turns free text into a label: no leading #, no spaces, no empty result. */
export function cleanLabel(name: string): string {
  return name.trim().replace(/^#+/, "").replace(/\s+/g, "-").replace(/[^\p{L}\p{N}_/-]/gu, "");
}

export function addLabel(text: string, name: string): string {
  const { plain, labels } = splitLabels(text);
  return joinLabels(plain, [...labels, name]);
}

export function removeLabel(text: string, name: string): string {
  const { plain, labels } = splitLabels(text);
  return joinLabels(plain, labels.filter((l) => l !== name));
}

/** True when the text would change by normalising label placement. */
export function needsNormalise(text: string): boolean {
  const { plain, labels } = splitLabels(text);
  return joinLabels(plain, labels) !== text.trim();
}

/** What the user is typing after a "#" at the caret, or null if not in a label. */
export function labelAtCaret(value: string, caret: number): { start: number; query: string } | null {
  const before = value.slice(0, caret);
  const m = before.match(/(?:^|\s)#([\p{L}\p{N}_/-]*)$/u);
  if (!m) return null;
  return { start: caret - m[1].length - 1, query: m[1] };
}
