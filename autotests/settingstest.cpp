/*
 * SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
 * SPDX-License-Identifier: MIT
 */

#include "settings.h"

#include <QDir>
#include <QFile>
#include <QJsonDocument>
#include <QJsonObject>
#include <QTemporaryDir>
#include <QTest>

using namespace Qt::StringLiterals;
using namespace NotaSettings;

class SettingsTest : public QObject
{
    Q_OBJECT

private Q_SLOTS:
    void init();
    void cleanup();

    void aMissingFileMeansAFirstLaunch();
    void absentKeysKeepTheirDefaults();
    void unrecognisedValuesAreNormalised();
    void legacyFontIdsBecomeFamilyNames();
    void unknownKeysSurviveASave();
    void pathsAreDerivedFromTheVault();
    void tildeIsExpandedOnlyInTheFormsThatMeanHome();

private:
    QString path() const
    {
        return m_dir->path() + "/settings.json"_L1;
    }
    void writeJson(const QString &json) const;

    std::unique_ptr<QTemporaryDir> m_dir;
};

void SettingsTest::init()
{
    m_dir = std::make_unique<QTemporaryDir>();
    QVERIFY(m_dir->isValid());
}

void SettingsTest::cleanup()
{
    m_dir.reset();
}

void SettingsTest::writeJson(const QString &json) const
{
    QFile f(path());
    QVERIFY(f.open(QIODevice::WriteOnly));
    f.write(json.toUtf8());
}

void SettingsTest::aMissingFileMeansAFirstLaunch()
{
    const Settings s = load(m_dir->path() + "/nothing-here.json"_L1);
    QCOMPARE(s.vaultPath, QDir::homePath() + "/Notes"_L1);
    QCOMPARE(s.workplanFolder, u"Workplans"_s);
    QVERIFY(s.createOnWeekends);
    QCOMPARE(s.theme, u"system"_s);
    QCOMPARE(s.split, u"rows"_s);
}

void SettingsTest::absentKeysKeepTheirDefaults()
{
    // A file written by an older version knows nothing about later keys; they
    // must land on their defaults rather than on empty values.
    writeJson(uR"({"vaultPath": "/tmp/somewhere"})"_s);

    const Settings s = load(path());
    QCOMPARE(s.vaultPath, u"/tmp/somewhere"_s);
    QCOMPARE(s.workplanFolder, u"Workplans"_s);
    QVERIFY(s.createOnWeekends);
    QCOMPARE(s.theme, u"system"_s);
}

void SettingsTest::unrecognisedValuesAreNormalised()
{
    writeJson(uR"({
      "theme": "solarized",
      "split": "diagonal",
      "workplanFolder": "",
      "fonts": {"size": "xxl"}
    })"_s);

    // A hand-edited or future value can never leave the interface broken.
    const Settings s = load(path());
    QCOMPARE(s.theme, u"system"_s);
    QCOMPARE(s.split, u"rows"_s);
    QCOMPARE(s.workplanFolder, u"Workplans"_s);
    QCOMPARE(s.fonts.size, u"m"_s);
}

void SettingsTest::legacyFontIdsBecomeFamilyNames()
{
    writeJson(uR"({"fonts": {"ui": "inter", "notes": "system", "code": "jetbrains-mono", "size": "l"}})"_s);

    const Settings s = load(path());
    QCOMPARE(s.fonts.ui, u"Inter"_s);
    // "system" meant "whatever the platform uses", which is now the default.
    QVERIFY(s.fonts.notes.isEmpty());
    QCOMPARE(s.fonts.code, u"JetBrains Mono"_s);
    QCOMPARE(s.fonts.size, u"l"_s);

    // And the translation is written back, so it happens exactly once.
    QVERIFY(save(path(), s));
    QCOMPARE(load(path()).fonts.ui, u"Inter"_s);
}

void SettingsTest::unknownKeysSurviveASave()
{
    // The Go application stored window geometry here. Kirigami owns that now,
    // but a vault shared between the two must not lose it.
    writeJson(uR"({
      "theme": "dark",
      "window": {"x": 10, "y": 20, "width": 1280, "height": 800, "maximised": true},
      "updates": {"check": "never"}
    })"_s);

    const Settings s = load(path());
    QCOMPARE(s.theme, u"dark"_s);
    QVERIFY(save(path(), s));

    QFile f(path());
    QVERIFY(f.open(QIODevice::ReadOnly));
    const QJsonObject root = QJsonDocument::fromJson(f.readAll()).object();

    QCOMPARE(root.value("window"_L1).toObject().value("width"_L1).toInt(), 1280);
    QVERIFY(root.value("window"_L1).toObject().value("maximised"_L1).toBool());
    QCOMPARE(root.value("updates"_L1).toObject().value("check"_L1).toString(), u"never"_s);
    QCOMPARE(root.value("theme"_L1).toString(), u"dark"_s);
}

void SettingsTest::pathsAreDerivedFromTheVault()
{
    Settings s;
    s.vaultPath = u"/home/someone/Notes"_s;

    QCOMPARE(s.appDir(), u"/home/someone/Notes/.nota"_s);
    QCOMPARE(s.settingsPath(), u"/home/someone/Notes/.nota/settings.json"_s);
    QCOMPARE(s.templatesDir(), u"/home/someone/Notes/.nota/templates"_s);
    QCOMPARE(s.workplanDir(), u"/home/someone/Notes/Workplans"_s);
}

void SettingsTest::tildeIsExpandedOnlyInTheFormsThatMeanHome()
{
    const QString home = QDir::homePath();

    QCOMPARE(expandHome(u"~"_s), home);
    QCOMPARE(expandHome(u"~/Notes"_s), home + "/Notes"_L1);
    // ~user is somebody else's home and is not ours to resolve.
    QCOMPARE(expandHome(u"~other/Notes"_s), u"~other/Notes"_s);
    QCOMPARE(expandHome(u"/absolute/Notes"_s), u"/absolute/Notes"_s);
    QCOMPARE(expandHome(u"relative/Notes"_s), u"relative/Notes"_s);
}

QTEST_GUILESS_MAIN(SettingsTest)

#include "settingstest.moc"
