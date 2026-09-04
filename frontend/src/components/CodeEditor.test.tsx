// @vitest-environment jsdom
import { cleanup, render } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

const storeImages = vi.fn(async (files: File[]) => files.map((f) => `![${f.name.replace(/\.[^.]+$/, "")}](attachments/x.png)`));
vi.mock("../lib/paste", async (orig) => {
  const real = await orig<typeof import("../lib/paste")>();
  return { ...real, storeImages: (f: File[]) => storeImages(f) };
});

const { CodeEditor } = await import("./CodeEditor");

afterEach(() => {
  cleanup();
  storeImages.mockClear();
});

function pasteImage(target: Element) {
  const file = new File([new Uint8Array([137, 80, 78, 71])], "shot.png", { type: "image/png" });
  const event = new Event("paste", { bubbles: true, cancelable: true }) as Event & { clipboardData: unknown };
  Object.defineProperty(event, "clipboardData", {
    value: { files: [file], items: [], getData: () => "" },
  });
  target.dispatchEvent(event);
  return event;
}

describe("pasting an image into an item's notes", () => {
  it("stores it and writes the markdown link", async () => {
    const onChange = vi.fn();
    const onError = vi.fn();
    const { container } = render(<CodeEditor value="" onChange={onChange} onError={onError} dark={false} />);
    const content = container.querySelector(".cm-content");
    expect(content).not.toBeNull();

    pasteImage(content as Element);
    await vi.waitFor(() => expect(storeImages).toHaveBeenCalledOnce());
    await vi.waitFor(() => expect(onChange).toHaveBeenCalled());
    // The insert must actually land: a decoration that throws used to abort it.
    expect(onError).not.toHaveBeenCalled();
    const last = onChange.mock.calls[onChange.mock.calls.length - 1];
    expect(String(last[0])).toContain("![shot](attachments/x.png)");
  });

  it("draws the picture under the line that names it", async () => {
    const { container } = render(
      <CodeEditor value={"before\n![shot](attachments/x.png)\nafter"} onChange={vi.fn()} dark={false} />,
    );
    await vi.waitFor(() => {
      const img = container.querySelector(".cm-nota-image img") as HTMLImageElement | null;
      expect(img).not.toBeNull();
      // The src is exactly what the markdown says, so the app serves it from
      // the vault at the same path any other editor would resolve.
      expect(img?.getAttribute("src")).toBe("attachments/x.png");
      expect(img?.alt).toBe("shot");
    });
  });

  it("keeps working while typing in notes that already hold a picture", async () => {
    // Block decorations from a plugin throw on every update, which broke both
    // typing and pasting in any note that had an image in it.
    const onChange = vi.fn();
    const onError = vi.fn();
    const { container } = render(
      <CodeEditor value={"![shot](attachments/x.png)"} onChange={onChange} onError={onError} dark={false} />,
    );
    const content = container.querySelector(".cm-content") as HTMLElement;
    pasteImage(content);
    await vi.waitFor(() => expect(onChange).toHaveBeenCalled());
    expect(onError).not.toHaveBeenCalled();
  });
});
