import { describe, expect, it } from "vitest";
import { DEFAULT_FONTS, FACES, faceFor, normaliseFonts } from "./fonts";

describe("fonts", () => {
  it("falls back to the first face of a slot for unknown ids", () => {
    expect(faceFor("ui", "comic-sans").id).toBe("inter");
    expect(faceFor("code", "").id).toBe("jetbrains-mono");
  });

  it("normalises a partial or bad settings object", () => {
    expect(normaliseFonts(null)).toEqual(DEFAULT_FONTS);
    expect(normaliseFonts({ ui: "manrope", notes: "nope", code: "system", size: "xl" as never })).toEqual({
      ui: "manrope", notes: "inter", code: "system", size: "m",
    });
  });

  it("offers System in every slot so nothing is forced on the user", () => {
    for (const slot of ["ui", "notes", "code"] as const) {
      expect(FACES[slot].some((f) => f.id === "system")).toBe(true);
    }
  });
});
