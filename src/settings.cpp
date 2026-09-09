/*
 * SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
 * SPDX-License-Identifier: MIT
 */

#include "settings.h"

#include <QDir>
#include <QFile>
#include <QJsonDocument>
#include <QSaveFile>
#include <QStandardPaths>

using namespace Qt::StringLiterals;

namespace NotaSettings
{
namespace
{

/*!
 * The Go application bundled six faces and named them by id. Those ids are
 * meaningless to a native app, so they are translated once, on read, into the
 * family names they stood for — and written back that way. A family that turns
 * out not to be installed falls back to the KDE font, which is what an empty
 * value means.
 */
QString familyForLegacyId(const QString &value)
{
    static const QHash<QString, QString> legacy = {
        {u"system"_s, QString()},
        {u"inter"_s, u"Inter"_s},
        {u"manrope"_s, u"Manrope"_s},
        {u"ibm-plex-sans"_s, u"IBM Plex Sans"_s},
        {u"lora"_s, u"Lora"_s},
        {u"source-serif-4"_s, u"Source Serif 4"_s},
        {u"jetbrains-mono"_s, u"JetBrains Mono"_s},
    };
    const auto found = legacy.constFind(value);
    if (found != legacy.cend()) {
        return *found;
    }
    return value;
}

QString normalisedFamily(const QString &value)
{
    // Whether the family is actually installed is a question for the GUI, which
    // has a font database; Qt falls back on its own for one that is not.
    return familyForLegacyId(value.trimmed());
}

QString normalisedSize(const QString &value)
{
    if (value == SizeSmall || value == SizeMedium || value == SizeLarge) {
        return value;
    }
    return QString(SizeMedium);
}

QString normalisedTheme(const QString &value)
{
    if (value == ThemeSystem || value == ThemeLight || value == ThemeDark) {
        return value;
    }
    // A hand-edited or future value must not leave the UI without a theme.
    return QString(ThemeSystem);
}

QString normalisedSplit(const QString &value)
{
    return value == SplitColumns ? QString(SplitColumns) : QString(SplitRows);
}

/*! The keys this version understands, so everything else can be kept aside. */
bool isKnownKey(const QString &key)
{
    static const QStringList known = {
        u"vaultPath"_s, u"workplanFolder"_s, u"createOnWeekends"_s, u"theme"_s, u"fonts"_s, u"split"_s,
        u"checkForUpdates"_s,
    };
    return known.contains(key);
}

} // namespace

QString Settings::appDir() const
{
    return vaultPath + "/.nota"_L1;
}

QString Settings::settingsPath() const
{
    return appDir() + "/settings.json"_L1;
}

QString Settings::templatesDir() const
{
    return appDir() + "/templates"_L1;
}

QString Settings::workplanDir() const
{
    const QString folder = workplanFolder.isEmpty() ? QString(DefaultWorkplanFolder) : workplanFolder;
    return vaultPath + u'/' + folder;
}

Settings defaults()
{
    Settings s;
    s.vaultPath = expandHome(u"~/Notes"_s);
    return s;
}

QString expandHome(const QString &path)
{
    if (path != "~"_L1 && !path.startsWith("~/"_L1)) {
        return path;
    }
    const QString home = QDir::homePath();
    if (home.isEmpty()) {
        return path;
    }
    if (path == "~"_L1) {
        return home;
    }
    return home + path.sliced(1);
}

Settings load(const QString &path)
{
    Settings s = defaults();

    QFile f(path);
    if (!f.open(QIODevice::ReadOnly)) {
        // A missing file means a first launch, not a problem.
        return s;
    }

    QJsonParseError error;
    const QJsonDocument doc = QJsonDocument::fromJson(f.readAll(), &error);
    if (error.error != QJsonParseError::NoError || !doc.isObject()) {
        return s;
    }
    const QJsonObject root = doc.object();

    if (root.contains("vaultPath"_L1)) {
        s.vaultPath = expandHome(root.value("vaultPath"_L1).toString(s.vaultPath));
    }
    if (root.contains("workplanFolder"_L1)) {
        const QString folder = root.value("workplanFolder"_L1).toString();
        s.workplanFolder = folder.isEmpty() ? QString(DefaultWorkplanFolder) : folder;
    }
    if (root.contains("createOnWeekends"_L1)) {
        s.createOnWeekends = root.value("createOnWeekends"_L1).toBool(s.createOnWeekends);
    }
    if (root.contains("checkForUpdates"_L1)) {
        s.checkForUpdates = root.value("checkForUpdates"_L1).toBool(s.checkForUpdates);
    }
    s.theme = normalisedTheme(root.value("theme"_L1).toString(s.theme));
    s.split = normalisedSplit(root.value("split"_L1).toString(s.split));

    const QJsonObject fonts = root.value("fonts"_L1).toObject();
    s.fonts.ui = normalisedFamily(fonts.value("ui"_L1).toString());
    s.fonts.notes = normalisedFamily(fonts.value("notes"_L1).toString());
    s.fonts.code = normalisedFamily(fonts.value("code"_L1).toString());
    s.fonts.size = normalisedSize(fonts.value("size"_L1).toString());

    for (auto it = root.constBegin(); it != root.constEnd(); ++it) {
        if (!isKnownKey(it.key())) {
            s.passthrough.insert(it.key(), it.value());
        }
    }
    return s;
}

bool save(const QString &path, const Settings &s)
{
    if (!QDir().mkpath(QFileInfo(path).absolutePath())) {
        return false;
    }

    QJsonObject root = s.passthrough;
    root.insert("vaultPath"_L1, s.vaultPath);
    root.insert("workplanFolder"_L1, s.workplanFolder);
    root.insert("createOnWeekends"_L1, s.createOnWeekends);
    root.insert("checkForUpdates"_L1, s.checkForUpdates);
    root.insert("theme"_L1, s.theme);
    root.insert("split"_L1, s.split);

    QJsonObject fonts;
    fonts.insert("ui"_L1, s.fonts.ui);
    fonts.insert("notes"_L1, s.fonts.notes);
    fonts.insert("code"_L1, s.fonts.code);
    fonts.insert("size"_L1, s.fonts.size);
    root.insert("fonts"_L1, fonts);

    QSaveFile out(path);
    if (!out.open(QIODevice::WriteOnly)) {
        return false;
    }
    out.write(QJsonDocument(root).toJson(QJsonDocument::Indented));
    if (!out.commit()) {
        return false;
    }
    // The file records where the user's notes are; it is nobody else's business.
    QFile::setPermissions(path, QFileDevice::ReadOwner | QFileDevice::WriteOwner);
    return true;
}

Settings resolve()
{
    const Settings probe = load(defaults().settingsPath());
    if (probe.vaultPath == defaults().vaultPath) {
        return probe;
    }
    // The settings at the default location point somewhere else; the real ones
    // live inside the vault they name.
    Settings real = load(probe.settingsPath());
    real.vaultPath = probe.vaultPath;
    return real;
}

} // namespace NotaSettings
