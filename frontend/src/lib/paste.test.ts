// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";

const saveAttachment = vi.fn(async (ext: string, data: string) => {
  void data;
  return `attachments/2026-x${ext}`;
});
vi.mock("./api", () => ({ api: { saveAttachment: (e: string, d: string) => saveAttachment(e, d) } }));

const { imagesIn, storeImage, storeImages } = await import("./paste");

function clipboard(files: File[]): DataTransfer {
  return { files, items: [] } as unknown as DataTransfer;
}
const png = (name = "shot.png") => new File([new Uint8Array([137, 80, 78, 71])], name, { type: "image/png" });

afterEach(() => saveAttachment.mockClear());

describe("imagesIn", () => {
  it("finds the images and ignores everything else", () => {
    const other = new File(["x"], "a.txt", { type: "text/plain" });
    expect(imagesIn(clipboard([png(), other])).map((f) => f.name)).toEqual(["shot.png"]);
  });

  it("is empty for a paste with no files, and for nothing at all", () => {
    expect(imagesIn(clipboard([]))).toEqual([]);
    expect(imagesIn(null)).toEqual([]);
  });
});

describe("storeImage", () => {
  it("stores the bytes and returns markdown pointing at the vault path", async () => {
    const md = await storeImage(png());
    // The path is relative to the vault, so the same link works outside Nota.
    expect(md).toBe("![shot](attachments/2026-x.png)");
    const call = saveAttachment.mock.calls[0];
    expect(call[0]).toBe(".png");
    // Bare base64, not a data URL: Go decodes it straight to bytes.
    expect(call[1]).not.toContain("data:");
    expect(call[1].length).toBeGreaterThan(0);
  });

  it("gives a clipboard image with no name something to be called", async () => {
    expect(await storeImage(png(""))).toBe("![image](attachments/2026-x.png)");
  });

  it("does not let a filename break the markdown link", async () => {
    expect(await storeImage(png("a[b].png"))).toBe("![ab](attachments/2026-x.png)");
  });

  it("keeps several pastes in the order they arrived", async () => {
    saveAttachment.mockImplementation(async (ext: string, data: string) => {
      void ext;
      void data;
      return `attachments/${saveAttachment.mock.calls.length}.png`;
    });
    const got = await storeImages([png("one.png"), png("two.png")]);
    expect(got).toEqual(["![one](attachments/1.png)", "![two](attachments/2.png)"]);
  });
});
