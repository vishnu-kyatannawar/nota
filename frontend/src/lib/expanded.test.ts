// @vitest-environment jsdom
import { act, cleanup, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { useExpanded } from "./expanded";

beforeEach(() => localStorage.clear());
afterEach(cleanup);

describe("useExpanded", () => {
  it("opens and shuts a folder, and persists the set", () => {
    const { result } = renderHook(() => useExpanded());
    act(() => result.current.toggle("A"));
    expect([...result.current.expanded]).toEqual(["A"]);
    expect(JSON.parse(localStorage.getItem("nota.expanded") ?? "[]")).toEqual(["A"]);

    act(() => result.current.toggle("A"));
    expect([...result.current.expanded]).toEqual([]);
  });

  it("restores what was persisted", () => {
    localStorage.setItem("nota.expanded", JSON.stringify(["A", "A/B"]));
    const { result } = renderHook(() => useExpanded());
    expect([...result.current.expanded].sort()).toEqual(["A", "A/B"]);
  });

  it("reveal opens every folder above a new node", () => {
    const { result } = renderHook(() => useExpanded());
    act(() => result.current.reveal("A/B/C/note.md"));
    expect([...result.current.expanded].sort()).toEqual(["A", "A/B", "A/B/C"]);
  });

  it("reveal of a root-level node changes nothing", () => {
    const { result } = renderHook(() => useExpanded());
    act(() => result.current.reveal("note.md"));
    expect([...result.current.expanded]).toEqual([]);
  });

  it("renamePrefix follows the folder and its descendants", () => {
    localStorage.setItem("nota.expanded", JSON.stringify(["A", "A/B", "Other"]));
    const { result } = renderHook(() => useExpanded());
    act(() => result.current.renamePrefix("A", "Z"));
    expect([...result.current.expanded].sort()).toEqual(["Other", "Z", "Z/B"]);
  });

  it("renamePrefix leaves a merely similar path alone", () => {
    localStorage.setItem("nota.expanded", JSON.stringify(["Abc"]));
    const { result } = renderHook(() => useExpanded());
    act(() => result.current.renamePrefix("A", "Z"));
    expect([...result.current.expanded]).toEqual(["Abc"]);
  });

  it("forget drops the path and everything under it", () => {
    localStorage.setItem("nota.expanded", JSON.stringify(["A", "A/B", "A/B/C", "Other"]));
    const { result } = renderHook(() => useExpanded());
    act(() => result.current.forget("A"));
    expect([...result.current.expanded]).toEqual(["Other"]);
  });
});

describe("expand", () => {
  it("opens a shut folder and leaves an open one open", () => {
    const { result } = renderHook(() => useExpanded());
    act(() => result.current.expand("A"));
    expect([...result.current.expanded]).toEqual(["A"]);
    act(() => result.current.expand("A"));
    expect([...result.current.expanded]).toEqual(["A"]);
  });
});
