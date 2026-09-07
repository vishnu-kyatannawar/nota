/*
 * SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
 * SPDX-License-Identifier: MIT
 *
 * Resolves where Nota keeps its data, and loads the user's settings.
 *
 * The vault (a directory of markdown files) is the source of truth for notes,
 * so everything here is about locating it. Settings themselves live inside the
 * vault at .nota/settings.json, which keeps a vault self-contained and
 * portable: copy the directory and the configuration travels with it.
 *
 * Window geometry is the one thing that does not live here. It is a property
 * of the machine rather than of the vault, and Kirigami's StatefulWindow
 * already persists it through KConfig — but the key is read and written back
 * untouched, so a vault shared with the Go application keeps its value.
 */

#pragma once

#include <QJsonObject>
#include <QString>

namespace NotaSettings
{

/*! The reserved folder holding one dated note per day. */
inline constexpr QLatin1StringView DefaultWorkplanFolder{"Workplans"};

/*! Theme values. "system" follows the Plasma colour scheme. */
inline constexpr QLatin1StringView ThemeSystem{"system"};
inline constexpr QLatin1StringView ThemeLight{"light"};
inline constexpr QLatin1StringView ThemeDark{"dark"};

/*! How the items and notes sections sit together when a page shows both. */
inline constexpr QLatin1StringView SplitRows{"rows"};
inline constexpr QLatin1StringView SplitColumns{"columns"};

/*! Font size steps. */
inline constexpr QLatin1StringView SizeSmall{"s"};
inline constexpr QLatin1StringView SizeMedium{"m"};
inline constexpr QLatin1StringView SizeLarge{"l"};

/*!
 * The faces the interface, the notes area and code use.
 *
 * An empty family means "whatever KDE is configured to use", which is the
 * default: overriding the user's own font settings is the opposite of native.
 * A family named here still travels with the vault.
 */
struct Fonts {
    QString ui;
    QString notes;
    QString code;
    QString size = QString(SizeMedium);
};

/*! The user's configuration, persisted as .nota/settings.json. */
struct Settings {
    /*! The root directory of the markdown vault, always absolute. */
    QString vaultPath;
    /*! The vault-relative folder holding the dated daily notes. */
    QString workplanFolder = QString(DefaultWorkplanFolder);
    /*!
     * Whether a workplan is created on Saturday and Sunday. Rollover copes
     * with gaps either way; this only decides whether a weekend gets a note.
     */
    bool createOnWeekends = true;
    /*! "system", "light" or "dark". */
    QString theme = QString(ThemeSystem);
    /*! The chosen typography. */
    Fonts fonts;
    /*! "rows" stacks items and notes, "columns" puts them side by side. */
    QString split = QString(SplitRows);

    /*!
     * Everything the file held that this version does not interpret, kept so
     * saving never drops a key another version wrote — the window geometry the
     * Go application stored included.
     */
    QJsonObject passthrough;

    /*! The vault-internal directory holding settings, templates and the trash. */
    QString appDir() const;
    /*! Where this configuration is persisted. */
    QString settingsPath() const;
    /*! Holds the recurring item templates. */
    QString templatesDir() const;
    /*! The absolute path of the reserved workplan folder. */
    QString workplanDir() const;
};

/*! The configuration used on a first launch. */
Settings defaults();

/*!
 * Resolves a leading ~ to the user's home directory. A bare path, an absolute
 * path and the ~user form are all returned unchanged — only the plain "~" and
 * "~/..." forms are expanded, which is what settings files realistically hold.
 */
QString expandHome(const QString &path);

/*!
 * Reads settings from \a path. A missing file is not an error: it means a first
 * launch, so the defaults are returned. Keys absent from the file fall back to
 * their defaults rather than landing as empty values, so adding a key in a
 * later version does not silently disable it for existing users, and every
 * unrecognised value is normalised back to its default so a hand-edited file
 * can never leave the interface broken.
 */
Settings load(const QString &path);

/*!
 * Writes settings to \a path, creating parent directories as needed. The write
 * goes through a temporary file and is then renamed, so an interrupted save
 * cannot leave a half-written settings file behind.
 */
bool save(const QString &path, const Settings &settings);

/*!
 * Finds the settings that apply.
 *
 * Bootstrapping is two-phase: probe the default location, and if the settings
 * found there point at a different vault, read the real ones from inside it.
 */
Settings resolve();

} // namespace NotaSettings
