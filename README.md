# Nota

A desktop note application built around one dated workplan per day.

The sidebar holds **pages**, grouped in folders. A page carries action items, free-form
notes, or both. Every page is a plain markdown file in a directory you own. That vault is
the source of truth — the app's SQLite index is a cache that can be deleted and rebuilt.
Open a page in vim, grep the whole tree, put it in git; none of that needs Nota running.

Linux and Windows, single binary, Go + a webview frontend.

**[nota website](https://vishnu-kyatannawar.github.io/nota/)** ·
[latest release](https://github.com/vishnu-kyatannawar/nota/releases/latest)

## Install

```sh
# Linux
curl -fsSL https://raw.githubusercontent.com/vishnu-kyatannawar/nota/main/install.sh | sh

# Windows
irm https://raw.githubusercontent.com/vishnu-kyatannawar/nota/main/install.ps1 | iex
```

Neither needs administrator rights, and both verify the download against the
release's published SHA-256 before installing it.

## What it does

- **Folders and nested folders** — the sidebar tree is the directory tree.
- **A workplan for every day**, named for its date, created automatically.
- **Rollover** — unfinished items move to the new day keeping their id, original
  creation time, labels, logged time, body and nesting; finished ones stay on the
  day they were finished. Monday carries from Friday, since the rule is "the most
  recent workplan", not "yesterday".
- **Hours worked per day**, shown as `2026-09-02 - 09:00`, sitting at `00:00` on a
  weekend, leave day or holiday. One figure per day, set by you.
- **Keyboard-first items** — type and press Enter, the checkbox appears for you. Arrow
  keys move between items; paste a list and each line becomes an item. Hover a row for
  one-click done, delete, and move-to-today.
- **Headings between items** — type `## Must` on an empty row to group what follows.
  Headings roll over with their open items and vanish when the group is finished.
- **Your fonts** — Inter, Manrope, IBM Plex Sans for the interface; Lora and Source Serif 4
  for notes; JetBrains Mono for code. All bundled, nothing fetched. Three sizes.
- **Light, dark or system theme**, and a window that opens maximised the first time and
  remembers its size and position after that.
- **Labels** — type `#label` inline on any item, or list them in a page's frontmatter.
- **Code and JSON** — fenced blocks with real syntax highlighting, in an item's
  notes or anywhere on the page.
- **Notes with formatting** — under the items on every workplan, and on any page: headings,
  bold, italics, bullets, numbered lists, quotes and code, saved as markdown.
- **Side by side, or stacked** — when a page shows both items and notes, a control in the page
  header puts them next to each other or one under the other. Workplans too.
- **Pages that are items, notes, or both** — an Items / Notes / Both toggle on every page.
  Keep a backlog as an item list and move items into today's workplan with one action.
- **Labels you can take off** — `#label` chips with an ×, and `#` autocompletes existing ones.
- **Trash** — deleted pages and folders sit in Trash for 30 days; restore them from the sidebar.
- **Recurring items** — daily, weekday or weekly, from a hand-editable file.
- **Images** — paste a screenshot into a page's notes or an item's notes. It is written to
  `attachments/` in your vault and linked as ordinary markdown, so it opens anywhere.
- **Export and restore** — one zip of the whole vault, and back again.
- **Updates, if you want them** — Nota can ask GitHub whether a newer version exists and install
  it for you, verifying the published SHA-256 first. It asks once before it ever uses the
  network, and answering no means it never does. This is the only request Nota makes.

### Keyboard

| Key | Action |
| --- | --- |
| `Enter` | New item below; at the start of an item, one above; mid-text, splits it |
| `Backspace` | At the start of an item, folds it into the one above |
| `↑` / `↓` | Move between items |
| `Backspace` on an empty item | Delete it and move up |
| `Tab` / `Shift+Tab` | Indent / outdent |
| `Ctrl+Enter` | Toggle done |
| `Ctrl+Backspace` / `Ctrl+Delete` | Delete the item |
| `## ` at the start of an empty row | Turn it into a heading |
| `#` | Add a label (autocompletes) |
| `Ctrl+Shift+N` | Open or remove the item's notes |
| `Ctrl+Shift+M` | Move the item to today's workplan |
| `Ctrl+E` | Swap the page to raw markdown and back |
| Paste several lines | One item per line; `- [x]` lines arrive ticked |

## Prerequisites

- Go 1.25+
- Node 24+ and pnpm 11+
- Wails v3 CLI: `go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.16`

Linux also needs the GTK4 and WebKitGTK 6 development packages:

| Distribution | Command |
| --- | --- |
| Debian / Ubuntu | `sudo apt install libgtk-4-dev libwebkitgtk-6.0-dev` |
| Fedora | `sudo dnf install gtk4-devel webkitgtk6.0-devel` |
| Arch | `sudo pacman -S gtk4 webkitgtk-6.0` |

## Development

```sh
cd frontend && pnpm install && cd ..

wails3 dev      # run with hot reload
wails3 build    # build bin/nota
```

`main.go` embeds `frontend/dist`, so the frontend must be built before the Go
binary. `wails3 build` and `wails3 dev` handle that ordering; a bare
`go build ./...` on a fresh clone needs `cd frontend && pnpm run build` first.

Checks, all of which CI runs on both Linux and Windows:

```sh
go test ./...                              # Go tests
golangci-lint run ./...                    # Go lint
cd frontend && pnpm run lint && pnpm run typecheck
```

Regenerate the frontend bindings after changing anything in `services/`:

```sh
wails3 generate bindings -d frontend/bindings
```

The generated bindings are committed, and CI fails if they drift from the Go source.

## Layout

```
main.go              application wiring
services/            types bound into the frontend; thin, delegate to internal/
internal/
  config/            settings and vault path resolution
  vault/             folder tree, note CRUD, file watching
  mdnote/            note format parse and serialise
  index/             SQLite index, search, labels
  workplan/          daily notes, rollover, recurring items
  export/            zip bundle export and restore
frontend/src/        React application
frontend/bindings/   generated by wails3; do not edit
build/               scaffolding emitted by `wails3 init`
```

Services stay deliberately thin. Wails v3 is still in beta, so keeping behaviour in
`internal/` means an API change upstream touches few files — and lets those packages
be tested without a webview.

## Data

The vault defaults to `~/Notes`, with settings inside it at `.nota/settings.json`,
so copying the directory carries the configuration with it.

```
~/Notes/
  Workplans/          reserved: one dated note per day
    2026-09-02.md
  Projects/           ordinary nested folders
  .nota/              settings, templates, index.db, trash/
```

A backup made with Export includes the trash, so a restore brings it back too.

A workplan looks like this. Labels stay visible where you would write them; only
ids and timestamps hide in a comment markdown does not render.

```markdown
---
type: workplan
date: 2026-09-02
hours: "01:20"
daytype: work
---

- [ ] Check calendar #daily <!--n id:01K6M2R0 t:08:55 rec:daily-->
- [x] Fix auth token expiry #rv-api <!--n id:01K6M2R4 t:09:34 done:11:02-->
      Middleware compares `exp < now`, off by one on the boundary second.

      ```go
      if exp <= now {
          return ErrExpired
      }
      ```

- [ ] Review PR 412 #rv-portal <!--n id:01K6J8XX t:09:40 from:2026-09-01 carried:1-->
```

Recurring items live in `.nota/templates/recurring.md`, one per line:

```markdown
- [ ] Check calendar #daily @daily
- [ ] Log the day bill #billing @weekdays
- [ ] Weekly report @weekly:fri
```

The filename stays the date alone. The hours are frontmatter, so logging time
never renames the file or churns git history.

## Licence

[MIT](LICENSE) — Copyright (c) 2026 Vishnu Kyatannawar.
