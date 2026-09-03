// Typed access to the Go services. Everything the UI knows about the vault
// arrives through here; the frontend never touches the filesystem itself.
import {
  AppService,
  NotesService,
  WorkplanService,
  SearchService,
  BackupService,
} from "../../bindings/github.com/vishnu-kyatannawar/nota/services";

export type Node = {
  name: string;
  path: string;
  isFolder: boolean;
  children?: Node[];
};

export type NoteItem = {
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
  labels: string[];
  items: NoteItem[];
  body: string;
  title: string;
};

export type Workplan = {
  path: string;
  date: string;
  hours: string;
  minutes: number;
  dayType: string;
  open: number;
  done: number;
};

export type Hit = { path: string; snippet: string };
export type Label = { name: string; count: number };
export type HoursSummary = { from: string; to: string; minutes: number; hours: string };

export const api = {
  info: () => AppService.GetInfo(),

  tree: () => NotesService.Tree() as Promise<Node>,
  note: (path: string) => NotesService.Get(path) as Promise<Note>,
  raw: (path: string) => NotesService.GetRaw(path),
  saveRaw: (path: string, content: string) => NotesService.SaveRaw(path, content),
  createNote: (path: string) => NotesService.Create(path),
  createFolder: (path: string) => NotesService.CreateFolder(path),
  rename: (from: string, to: string) => NotesService.Rename(from, to),
  remove: (path: string) => NotesService.Delete(path),

  ensureToday: () => WorkplanService.EnsureToday(),
  workplans: () => WorkplanService.List() as Promise<Workplan[]>,
  setHours: (path: string, hours: string) => WorkplanService.SetHours(path, hours),
  setDayType: (path: string, dayType: string) => WorkplanService.SetDayType(path, dayType),
  suggestedMinutes: (path: string) => WorkplanService.SuggestedMinutes(path),
  addItem: (path: string, text: string, depth: number) => WorkplanService.AddItem(path, text, depth),
  setItemDone: (path: string, id: string, done: boolean) => WorkplanService.SetItemDone(path, id, done),
  setItemText: (path: string, id: string, text: string) => WorkplanService.SetItemText(path, id, text),
  addItemMinutes: (path: string, id: string, minutes: number) =>
    WorkplanService.AddItemMinutes(path, id, minutes),
  removeItem: (path: string, id: string) => WorkplanService.RemoveItem(path, id),
  setItemBody: (path: string, id: string, body: string[]) =>
    WorkplanService.SetItemBody(path, id, body),

  search: (query: string) => SearchService.Search(query) as Promise<Hit[]>,
  labels: () => SearchService.Labels() as Promise<Label[]>,
  notesByLabel: (name: string) => SearchService.NotesByLabel(name) as Promise<string[]>,
  hoursThisWeek: () => SearchService.HoursThisWeek() as Promise<HoursSummary>,

  bundleName: () => BackupService.DefaultBundleName(),
  exportTo: (dir: string) => BackupService.Export(dir),
  restore: (bundle: string) => BackupService.Restore(bundle),
};

/** Splits an item's text into the parts the row renders separately. */
export function partsOf(text: string) {
  const labels = [...text.matchAll(/#([\p{L}\p{N}][\p{L}\p{N}_/-]*)/gu)].map((m) => m[1]);
  const time = text.match(/\[(\d{2}:[0-5]\d)\]/);
  const plain = text
    .replace(/\[(\d{2}:[0-5]\d)\]/g, "")
    .replace(/#([\p{L}\p{N}][\p{L}\p{N}_/-]*)/gu, "")
    .replace(/\s+/g, " ")
    .trim();
  return { plain, labels, time: time ? time[1] : "" };
}

/** Formats minutes as hh:mm, the only time format the app uses. */
export function hhmm(minutes: number): string {
  const m = Math.max(0, minutes);
  return `${String(Math.floor(m / 60)).padStart(2, "0")}:${String(m % 60).padStart(2, "0")}`;
}
