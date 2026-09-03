/**
 * The bundled typefaces and how the chosen ones are applied. The woff2 files
 * ship inside the application (imported in main.tsx), so no font is ever
 * fetched over the network.
 */
export type FontSlot = "ui" | "notes" | "code";
export type FontSize = "s" | "m" | "l";
export type Fonts = { ui: string; notes: string; code: string; size: FontSize };

export type Face = { id: string; label: string; stack: string; sample: string };

const SYSTEM_SANS = `ui-sans-serif, system-ui, -apple-system, "Segoe UI", Roboto, sans-serif`;
const SYSTEM_SERIF = `ui-serif, Georgia, "Times New Roman", serif`;
const SYSTEM_MONO = `ui-monospace, SFMono-Regular, Menlo, Consolas, monospace`;

export const FACES: Record<FontSlot, Face[]> = {
  ui: [
    { id: "inter", label: "Inter", stack: `"Inter Variable", ${SYSTEM_SANS}`, sample: "Clean and neutral" },
    { id: "manrope", label: "Manrope", stack: `"Manrope Variable", ${SYSTEM_SANS}`, sample: "Rounded and modern" },
    { id: "ibm-plex-sans", label: "IBM Plex Sans", stack: `"IBM Plex Sans Variable", ${SYSTEM_SANS}`, sample: "Crisp and technical" },
    { id: "system", label: "System", stack: SYSTEM_SANS, sample: "Whatever your desktop uses" },
  ],
  notes: [
    { id: "inter", label: "Inter", stack: `"Inter Variable", ${SYSTEM_SANS}`, sample: "Same as the interface" },
    { id: "lora", label: "Lora", stack: `"Lora Variable", ${SYSTEM_SERIF}`, sample: "A warm serif for reading" },
    { id: "source-serif-4", label: "Source Serif 4", stack: `"Source Serif 4 Variable", ${SYSTEM_SERIF}`, sample: "A classic book serif" },
    { id: "system", label: "System", stack: SYSTEM_SANS, sample: "Whatever your desktop uses" },
  ],
  code: [
    { id: "jetbrains-mono", label: "JetBrains Mono", stack: `"JetBrains Mono Variable", ${SYSTEM_MONO}`, sample: "if exp <= now { }" },
    { id: "system", label: "System mono", stack: SYSTEM_MONO, sample: "if exp <= now { }" },
  ],
};

export const SIZES: { id: FontSize; label: string; px: number }[] = [
  { id: "s", label: "Small", px: 13 },
  { id: "m", label: "Medium", px: 14 },
  { id: "l", label: "Large", px: 16 },
];

export const DEFAULT_FONTS: Fonts = { ui: "inter", notes: "inter", code: "jetbrains-mono", size: "m" };

export function faceFor(slot: FontSlot, id: string): Face {
  return FACES[slot].find((f) => f.id === id) ?? FACES[slot][0];
}

export function normaliseFonts(f: Partial<Fonts> | null | undefined): Fonts {
  return {
    ui: faceFor("ui", f?.ui ?? "").id,
    notes: faceFor("notes", f?.notes ?? "").id,
    code: faceFor("code", f?.code ?? "").id,
    size: SIZES.some((s) => s.id === f?.size) ? (f!.size as FontSize) : "m",
  };
}

/** Writes the chosen faces into the CSS variables the stylesheet reads. */
export function applyFonts(f: Fonts) {
  const root = document.documentElement.style;
  root.setProperty("--font-sans", faceFor("ui", f.ui).stack);
  root.setProperty("--font-serif", faceFor("notes", f.notes).stack);
  root.setProperty("--font-mono", faceFor("code", f.code).stack);
  root.setProperty("--font-size-base", `${SIZES.find((s) => s.id === f.size)?.px ?? 14}px`);
}
