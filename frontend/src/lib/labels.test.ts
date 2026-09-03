import { describe, expect, it } from "vitest";
import { addLabel, cleanLabel, joinLabels, labelAtCaret, needsNormalise, removeLabel, splitLabels } from "./labels";

describe("splitLabels / joinLabels", () => {
  it("separates trailing labels from the text", () => {
    expect(splitLabels("Review PR 412 #rv-portal #urgent")).toEqual({ plain: "Review PR 412", labels: ["rv-portal", "urgent"] });
  });

  it("finds labels written mid-text and moves them to the tail on join", () => {
    const parts = splitLabels("Fix the #rv-api bug today");
    expect(parts).toEqual({ plain: "Fix the bug today", labels: ["rv-api"] });
    expect(joinLabels(parts.plain, parts.labels)).toBe("Fix the bug today #rv-api");
  });

  it("deduplicates and keeps order", () => {
    expect(joinLabels("x", ["b", "a", "b"])).toBe("x #b #a");
  });

  it("round-trips already-canonical text unchanged", () => {
    const text = "Chase the invoice #billing";
    const { plain, labels } = splitLabels(text);
    expect(joinLabels(plain, labels)).toBe(text);
    expect(needsNormalise(text)).toBe(false);
    expect(needsNormalise("Fix the #rv-api bug")).toBe(true);
  });

  it("handles labels-only and empty text", () => {
    expect(joinLabels("", ["a"])).toBe("#a");
    expect(joinLabels("", [])).toBe("");
    expect(splitLabels("#only")).toEqual({ plain: "", labels: ["only"] });
  });
});

describe("add / remove", () => {
  it("adds without duplicating", () => {
    expect(addLabel("Task #a", "a")).toBe("Task #a");
    expect(addLabel("Task #a", "#b")).toBe("Task #a #b");
    expect(addLabel("Task", "two words")).toBe("Task #two-words");
  });

  it("removes only the named label", () => {
    expect(removeLabel("Task #a #b", "a")).toBe("Task #b");
    expect(removeLabel("Task #a", "a")).toBe("Task");
    expect(removeLabel("Task #a", "zzz")).toBe("Task #a");
  });
});

describe("cleanLabel", () => {
  it("strips # and punctuation, joins spaces", () => {
    expect(cleanLabel("  #Work Plan!  ")).toBe("Work-Plan");
    expect(cleanLabel("###")).toBe("");
  });
});

describe("labelAtCaret", () => {
  it("reports the label being typed at the caret", () => {
    expect(labelAtCaret("Fix bug #rv", 11)).toEqual({ start: 8, query: "rv" });
    expect(labelAtCaret("Fix bug #", 9)).toEqual({ start: 8, query: "" });
  });

  it("is null when not in a label", () => {
    expect(labelAtCaret("Fix bug", 7)).toBeNull();
    expect(labelAtCaret("Fix bug #rv done", 16)).toBeNull();
    expect(labelAtCaret("a#b", 3)).toBeNull(); // # must start a word
  });
});
