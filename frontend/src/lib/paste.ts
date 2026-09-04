/**
 * Pasted images.
 *
 * A clipboard image has no name and no path — it is bytes. Go writes it into
 * the vault's attachments folder and hands back the path a note should refer
 * to, which is relative to the vault root so the same markdown link works in
 * the app, in another editor, and in the file tree.
 */

import { api } from "./api";

const EXT: Record<string, string> = {
  "image/png": ".png",
  "image/jpeg": ".jpg",
  "image/gif": ".gif",
  "image/webp": ".webp",
  "image/svg+xml": ".svg",
  "image/avif": ".avif",
  "image/bmp": ".bmp",
};

/** The image files on a clipboard or drop, ignoring the text that came with them. */
export function imagesIn(data: DataTransfer | null): File[] {
  if (!data) return [];
  return Array.from(data.files).filter((f) => EXT[f.type] !== undefined);
}

function base64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onerror = () => reject(new Error(`could not read ${file.name || "the pasted image"}`));
    reader.onload = () => {
      const result = String(reader.result);
      resolve(result.slice(result.indexOf(",") + 1));
    };
    reader.readAsDataURL(file);
  });
}

/** Stores one image and returns the markdown that shows it. */
export async function storeImage(file: File): Promise<string> {
  const path = await api.saveAttachment(EXT[file.type] ?? ".png", await base64(file));
  const alt = (file.name || "image").replace(/\.[^.]+$/, "").replace(/[[\]]/g, "");
  return `![${alt}](${path})`;
}

/** Stores several, in the order they were pasted. */
export async function storeImages(files: File[]): Promise<string[]> {
  const out: string[] = [];
  for (const f of files) out.push(await storeImage(f));
  return out;
}
