// Typed access to the Go services. Everything the UI knows about the vault
// arrives through here; the frontend never touches the filesystem itself.
import {
  AppService,
  NotesService,
  WorkplanService,
  SearchService,
  BackupService,
  UpdateService,
  AttachmentService,
} from "../../bindings/github.com/vishnu-kyatannawar/nota/services";
import type { Theme } from "./theme";
import type { Fonts } from "./fonts";
import type { ItemInput } from "./items";

export type Node = { name: string; path: string; isFolder: boolean; children?: Node[] };

export type NoteItem = {
  kind: string;
  level: number;
  id: string;
  text: string;
  done: boolean;
  depth: number;
  minutes: number;
  labels: string[];
  createdAt: string;
  doneAt: string;
  from: string;
  carried: number;
  recurring: string;
  body: string[];
};

export type Note = {
  path: string;
  id: string;
  type: string;
  date: string;
  hours: string;
  dayType: string;
  layout: "items" | "notes" | "both";
  labels: string[];
  items: NoteItem[];
  body: string;
  title: string;
};

export type TrashEntry = { id: string; path: string; name: string; isFolder: boolean; deletedAt: string };

export type Workplan = {
  path: string;
  date: string;
  hours: string;
  minutes: number;
  dayType: string;
  open: number;
  done: number;
};

export type SavedItem = {
  Kind: string; Level: number; ID: string; Text: string; Done: boolean; Depth: number; Body: string[] | null;
  CreatedAt: string; DoneAt: string; From: string; Carried: number; Recurring: string;
};

export type Hit = { path: string; snippet: string };
export type Label = { name: string; count: number };
export type HoursSummary = { from: string; to: string; minutes: number; hours: string };
export type Info = {
  version: string; vaultPath: string; workplanDir: string; theme: Theme; fonts: Fonts;
  repository: string; website: string; releases: string; licence: string;
  /** "ask" until the user has answered whether the app may check for updates. */
  updateCheck: UpdateCheck;
};

export type UpdateCheck = "ask" | "auto" | "never";

/** What the update banner renders from; see services/update.go. */
export type UpdateState = {
  phase: "idle" | "checking" | "current" | "available" | "downloading" | "ready" | "failed";
  version: string;
  percent: number;
  message: string;
  /** False when the binary is not this user's to replace. */
  canInstall: boolean;
  releaseUrl: string;
};
export type Settings = {
  vaultPath: string; workplanFolder: string; createOnWeekends: boolean; theme: Theme;
};

export const api = {
  info: () => AppService.GetInfo() as Promise<Info>,
  settings: () => AppService.GetSettings() as Promise<Settings>,
  saveSettings: (s: Settings) => AppService.SaveSettings(s as never),
  setTheme: (theme: Theme) => AppService.SetTheme(theme),
  setFonts: (fonts: Fonts) => AppService.SetFonts(fonts as never),

  updateState: () => UpdateService.State() as Promise<UpdateState>,
  checkUpdate: (manual: boolean) => UpdateService.Check(manual) as Promise<UpdateState>,
  installUpdate: () => UpdateService.Install() as Promise<UpdateState>,
  setUpdateCheck: (check: UpdateCheck) => UpdateService.SetPreference(check),

  /** Stores a pasted image and returns the vault path to put in the markdown. */
  saveAttachment: (ext: string, base64Data: string) => AttachmentService.Save(ext, base64Data),

  tree: () => NotesService.Tree() as Promise<Node>,
  note: (path: string) => NotesService.Get(path) as Promise<Note>,
  raw: (path: string) => NotesService.GetRaw(path),
  saveRaw: (path: string, content: string) => NotesService.SaveRaw(path, content),
  createNote: (path: string) => NotesService.Create(path),
  createFolder: (path: string) => NotesService.CreateFolder(path),
  rename: (from: string, to: string) => NotesService.Rename(from, to),
  remove: (path: string) => NotesService.Delete(path),
  saveBody: (path: string, body: string) => NotesService.SaveBody(path, body),
  setLayout: (path: string, layout: string) => NotesService.SetLayout(path, layout),
  setLabels: (path: string, labels: string[]) => NotesService.SetLabels(path, labels),
  listTrash: () => NotesService.ListTrash() as Promise<TrashEntry[]>,
  restore: (id: string) => NotesService.Restore(id),
  deleteForever: (id: string) => NotesService.DeleteForever(id),
  emptyTrash: () => NotesService.EmptyTrash(),
  moveItems: (from: string, ids: string[], to: string) => WorkplanService.MoveItems(from, ids, to),

  ensureToday: () => WorkplanService.EnsureToday(),
  workplans: () => WorkplanService.List() as Promise<Workplan[]>,
  setHours: (path: string, hours: string) => WorkplanService.SetHours(path, hours),
  setDayType: (path: string, dayType: string) => WorkplanService.SetDayType(path, dayType),
  saveItems: (path: string, items: ItemInput[]) =>
    WorkplanService.SaveItems(path, items as never) as Promise<SavedItem[]>,

  search: (query: string) => SearchService.Search(query) as Promise<Hit[]>,
  labels: () => SearchService.Labels() as Promise<Label[]>,
  notesByLabel: (name: string) => SearchService.NotesByLabel(name) as Promise<string[]>,
  hoursThisWeek: () => SearchService.HoursThisWeek() as Promise<HoursSummary>,

  bundleName: () => BackupService.DefaultBundleName(),
  exportTo: (dir: string) => BackupService.Export(dir),
  restoreBackup: (bundle: string) => BackupService.Restore(bundle),
};

/** The #labels written in an item's text, and the text with them removed. */
export function partsOf(text: string) {
  const labels = [...text.matchAll(/#([\p{L}\p{N}][\p{L}\p{N}_/-]*)/gu)].map((m) => m[1]);
  return { labels };
}

/** Formats minutes as hh:mm. */
export function hhmm(minutes: number): string {
  const m = Math.max(0, minutes);
  return `${String(Math.floor(m / 60)).padStart(2, "0")}:${String(m % 60).padStart(2, "0")}`;
}

/** Validates the only time format the app accepts. */
export const HHMM = /^\d{2,}:[0-5]\d$/;
