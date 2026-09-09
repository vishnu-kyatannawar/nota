# CLAUDE.md

Guidance for Claude Code working in this repository.

## What this is

Nota: a KDE/Kirigami note application built around one dated workplan per day. Qt 6.9+,
KDE Frameworks 6.10+, C++20, CMake with extra-cmake-modules. Linux only.

The vault — a directory of plain markdown files, `~/Notes` by default — is the source of
truth. Anything the application keeps beside it is a cache that may be deleted and rebuilt.

## Build and test

```sh
cmake -B build -G Ninja -DCMAKE_BUILD_TYPE=Debug -DBUILD_TESTING=ON
cmake --build build
ctest --test-dir build --output-on-failure
```

Tests are QTest suites under `autotests/`, named `nota-*`. Run one with
`ctest --test-dir build -R nota-mdnote --output-on-failure`. The QML suite
(`nota-qmltest`) drives the real components offscreen; CI sets `QT_QPA_PLATFORM=offscreen`
for the whole run.

CI (`.github/workflows/ci.yml`) builds in an `archlinux:base-devel` container, because no
Ubuntu runner image carries Qt 6.9 / KF 6.10. Anything that must pass CI must build there.

## Targets

| Target | Contents | Links |
| --- | --- | --- |
| `notacore` | `mdnote`, `vault`, `workplan`, `settings` | Qt Core, KF6::CoreAddons, KF6::I18n |
| `notaqml` | `app`, `itemmodel`, `foldertreemodel`, `qml/` — QML module `org.kde.nota` | `notacore`, Qt Quick, KConfig, KItemModels, KSyntaxHighlighting |
| `nota` | `main.cpp` only | `notaqml`, KirigamiApp |

`notacore` has no QML and no GUI, deliberately: the parser, the rollover rules and the
settings are the parts worth testing, and none of them should need a window. Keep it that
way — new behaviour belongs there, not in `app.cpp`.

The QML module lives in the `notaqml` **library**, not in the executable, so tests can link
it and drive the real components. A statically linked QML module needs
`qt_import_qml_plugins()` on every consumer or the engine reports the module as missing at
import time.

## Invariants

- **Round-tripping is `mdnote`'s whole promise.** The parser is line-based, not an AST: the
  bytes it did not interpret are the bytes it writes back. Item text is stored verbatim —
  `labels()` and `minutes()` are derived views over it, never a replacement. Golden files in
  `autotests/data/` must round-trip byte for byte.
- **The vault format is frozen.** An existing vault written by the old Go build must open
  unchanged, and files written here must stay byte-for-byte compatible with it.
- **`vault.cpp` is the only code that touches the notes directory.** Every path crossing
  that boundary is vault-relative and validated; a path escaping the root would read or
  write arbitrary files.
- **`Workplan::ensure()` is idempotent.** It runs at launch, on a midnight timer and on
  window focus, so repeating it must never duplicate an item or disturb an edited day.
- **`ItemModel` is a flat list with a `DepthRole`,** not a tree, because that is what the
  file format is. Headings occupy a row (`IsHeadingRole`), so model row and view row are the
  same number — the focus handling depends on it.
- **Never call `setItems()` on a save.** A save runs every 400 ms while someone types, and
  the model reset destroys the delegate being typed in. Use `mergeSaved()`.
- **`FolderTreeModel::refresh()` must never reset the model.** It diffs the vault against
  the tree it holds and reports the difference as row insertions and removals. A reset is
  what collapses every folder the user had expanded — the view cannot tell that the node it
  had open is the same node afterwards. `sidebartreetest.cpp` asserts no reset is emitted
  for a create, a delete or a rename, with `QAbstractItemModelTester` checking the row maths.
- **`syncChildren()` relies on `Vault::tree()`'s ordering, not on a copy of its comparator.**
  Survivors keep their relative order because both lists come out of the same sort, which is
  all the algorithm needs. Do not reimplement folders-before-notes here.
- **A QML property must not collide with a superclass member.** `Kirigami.Dialog` carries a
  FINAL `result` and a `done()` signal; shadowing either fails the whole component, so the
  window never gets created and the app exits 1 in silence. `autotests/windowtest.cpp`
  exists to catch exactly that.
- **Qt routes warnings to the journal, not the terminal.** When the app dies quietly, rerun
  it with `QT_FORCE_STDERR_LOGGING=1` before assuming there is no error message.
- **The update check needs a `User-Agent`.** GitHub's API answers 403 without one and Qt
  sets none, so dropping that header makes the check fail silently everywhere.
- **Only the workplan folder itself is reserved.** The dated notes under it can be deleted
  like any page; they cannot be *renamed*, because the filename is the date. `isReserved()`
  is the folder, `isDatedPage()` is a note under it.
- Settings live in the vault at `.nota/settings.json` so a vault stays portable. Window
  geometry is the exception: it is machine state, persisted through KConfig.

## Conventions

- Every source file starts with the SPDX header:
  `SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar` / `SPDX-License-Identifier: MIT`.
- Header comments explain *why* the design is what it is; match that register rather than
  restating what the code does.
- KDE compiler settings are strict (`KDECompilerSettings`) — no implicit `QString` casts
  from `const char *`. Use `QLatin1StringView` / `QStringLiteral`. Deprecations are disabled
  up to Qt 6.9 / KF 6.10.
- User-visible strings go through `i18n()`; `TRANSLATION_DOMAIN` is `"nota"`.
- Shortcuts are Kirigami actions, so they appear in the command bar and can be rebound.
- Commits follow Conventional Commits. Work on `main`.

## Packaging

`PKGBUILD` builds the development branch as `nota-git`. Running `makepkg` in the repo root
leaves `nota-git/`, `pkg/`, `src/nota-git/` and `*.pkg.tar.zst` behind — all git-ignored —
and rewrites `pkgver=` in place from `pkgver()`. `install.sh` is the from-source one-line
install into `~/.local`.
