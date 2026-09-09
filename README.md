# Nota

A note application built around one dated workplan per day, native to KDE Plasma.

The sidebar holds **pages**, grouped in folders. A page carries action items, free-form
notes, or both. Every page is a plain markdown file in a directory you own. That vault is
the source of truth — anything the app keeps beside it is a cache that can be deleted and
rebuilt. Open a page in Kate, grep the whole tree, put it in git; none of that needs Nota
running.

Qt 6 and KDE Frameworks 6, so it follows your Breeze colour scheme, your fonts and your
shortcuts rather than bringing its own.

**[nota website](https://vishnu-kyatannawar.github.io/nota/)** ·
[latest release](https://github.com/vishnu-kyatannawar/nota/releases/latest)

## Where this is

Nota has been rewritten. It was a Go application drawing its interface in an embedded
WebKitGTK view; **v5.0.0 is a native Kirigami one**. The vault format has not changed and
will not: an existing `~/Notes` opens in the new build unchanged, and the files it writes
are byte-for-byte what the old one wrote.

| | State |
| --- | --- |
| Note format, vault, settings, rollover, repeating items | Ported to C++, 124 tests |
| The interface | Kirigami, keyboard-first |
| Search, image paste, export/restore, self-update | Not ported yet |

**v5.0.0 is Linux only, and it does not update itself.** The in-app updater was part of the
Go build; reinstall with the one-liner below to move between versions. Windows ends at
v4.6.1.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/vishnu-kyatannawar/nota/main/install.sh | sh
```

No root: everything lands under `~/.local`. The script checks for Qt 6 and every framework
first and prints your distribution's install line if any are missing, rather than leaving
CMake to fail halfway.

It builds rather than downloads, and that is deliberate. A KF6 binary links the libraries
its own distribution ships, so a prebuilt tarball would fail to start on a good share of the
machines that ran it. Building against the frameworks you already have is the honest version
of a one-line install for this kind of application.

On Arch and CachyOS the packaged route is better, since pacman then owns the files:

```sh
curl -fsSLO https://raw.githubusercontent.com/vishnu-kyatannawar/nota/main/PKGBUILD &&
makepkg -si
```

## Build it yourself

```sh
git clone https://github.com/vishnu-kyatannawar/nota.git
cd nota
cmake -B build -G Ninja -DCMAKE_BUILD_TYPE=Debug
cmake --build build
ctest --test-dir build --output-on-failure
```

Requires Qt 6.9+, KDE Frameworks 6.10+ and extra-cmake-modules.

| Distribution | Command |
| --- | --- |
| Arch / CachyOS | `sudo pacman -S --needed cmake extra-cmake-modules ninja qt6-base qt6-declarative kirigami kirigami-addons ki18n kcoreaddons kconfig kcrash kitemmodels syntax-highlighting kcolorscheme kiconthemes qqc2-desktop-style breeze-icons` |
| Fedora | `sudo dnf install cmake extra-cmake-modules ninja-build qt6-qtbase-devel qt6-qtdeclarative-devel kf6-kirigami-devel kf6-kirigami-addons-devel kf6-ki18n-devel kf6-kcoreaddons-devel kf6-kconfig-devel kf6-kcrash-devel kf6-kitemmodels-devel kf6-syntax-highlighting-devel qqc2-desktop-style` |
| Debian / Ubuntu | `sudo apt install cmake extra-cmake-modules ninja-build qt6-base-dev qt6-declarative-dev libkf6kirigami-dev libkf6i18n-dev libkf6coreaddons-dev libkf6config-dev libkf6crash-dev libkf6itemmodels-dev libkf6syntaxhighlighting-dev qml6-module-org-kde-kirigami qqc2-desktop-style` |

Linux only. The previous release ran on Windows; a Kirigami application there would mean
bundling Qt and every framework for a window with no Breeze and no Plasma integration,
which is the opposite of the point. Windows users should stay on v4.6.1.

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
  keys move between items; paste a list and each line becomes an item.
- **Headings between items** — type `## Must` on an empty row to group what follows.
  Headings roll over with their open items and vanish when the group is finished.
- **Items that repeat** — a section at the top of every workplan. Add one there and it
  comes back each day unticked; stopping it leaves your past workplans untouched.
- **Labels** — type `#label` inline on any item, or list them in a page's frontmatter.
- **Notes with formatting** — under the items on every workplan, and on any page.
- **Side by side, or stacked** — when a page shows both items and notes, a control in the
  page header puts them next to each other or one under the other.
- **Pages that are items, notes, or both** — an Items / Notes / Both toggle on every page.
- **Trash** — deleted pages and folders sit in Trash rather than being removed.
- **Your desktop's look** — Breeze colours, your KDE fonts, your icon theme. A font
  override per slot is available and travels with the vault, but the default is whatever
  Plasma is set to.

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
| `#` | Add a label |
| `Ctrl+Shift+N` | Open or remove the item's notes |
| `Ctrl+Shift+M` | Move the item to today's workplan |
| `Ctrl+E` | Swap the page to raw markdown and back |
| Paste several lines | One item per line; `- [x]` lines arrive ticked |

Shortcuts are Kirigami actions, so they show up in the command bar and can be rebound.

## Layout

```
CMakeLists.txt       ECM + KDE Frameworks, one target for the logic and one for the app
src/
  mdnote.{h,cpp}     the note format: parse and serialise, byte for byte
  vault.{h,cpp}      folder tree, note CRUD, atomic writes, KDirWatch
  workplan.{h,cpp}   daily notes, rollover, recurring items
  settings.{h,cpp}   .nota/settings.json
  qml/               the Kirigami interface
autotests/           QTest suites
autotests/data/      the golden files the parser must round-trip
icons/               the application icon
```

`notacore` is a plain library linking only Qt Core and KCoreAddons — no QML, no GUI. That
is deliberate: the parser, the rollover rules and the settings are the parts worth testing,
and none of them should need a window to run.

## Data

The vault defaults to `~/Notes`, with settings inside it at `.nota/settings.json`,
so copying the directory carries the configuration with it.

```
~/Notes/
  Workplans/          reserved: one dated note per day
    2026-09-02.md
  Projects/           ordinary nested folders
  .nota/              settings, templates, trash/
```

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

The parser is line-based rather than built on a markdown AST, because round-tripping a note
without changing it is the whole promise: the bytes it did not interpret are the bytes it
writes back. A fenced code block inside an item's notes may contain `- [ ]` lines, and they
are not items.

Items that repeat every day sit in their own section at the top of each workplan.
Add one there, and it appears today and every day after, unticked each morning
whether or not you finished it yesterday. Editing the text renames it everywhere.
Stopping one takes it out of today and tomorrow; workplans already written keep
their copy, because what you did on a day is a record of that day.

They are kept in `.nota/templates/recurring.md`, which you can also edit by hand —
that is the only way to get the weekday-only and weekly cadences:

```markdown
- [ ] Check calendar #daily @daily
- [ ] Log the day bill #billing @weekdays
- [ ] Weekly report @weekly:fri
```

The filename stays the date alone. The hours are frontmatter, so logging time
never renames the file or churns git history.

Window geometry is the one setting that does not live in the vault. It belongs to the
machine rather than to the notes, and Kirigami persists it through KConfig — but the key
the Go build wrote is read and written back untouched, so a vault shared between the two
loses nothing.

## Licence

[MIT](LICENSE) — Copyright (c) 2026 Vishnu Kyatannawar.
