/*
 * SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
 * SPDX-License-Identifier: MIT
 *
 * That the window can be created at all.
 *
 * Every other suite drives a component in isolation, which is why a Main.qml
 * that cannot be instantiated once shipped past all of them: a property whose
 * name collides with a FINAL member of a superclass fails the whole component,
 * and Qt routes that message to the journal rather than the terminal, so the
 * application simply exits with status 1 and says nothing at all.
 */

#include "app.h"

#include <KConfigGroup>
#include <KLocalizedQmlContext>
#include <KSharedConfig>
#include <KLocalizedString>

#include <QDir>
#include <QFile>
#include <QQmlApplicationEngine>
#include <QQuickItem>
#include <QStandardPaths>
#include <QQmlError>
#include <QTemporaryDir>
#include <QTest>

using namespace Qt::StringLiterals;

class WindowTest : public QObject
{
    Q_OBJECT

private Q_SLOTS:
    void theMainWindowIsCreatedWithoutQmlErrors();
    void theThreeColumnsAreThereAndCanBeResized();
    void aDraggedWidthOutlivesTheSession();
};

void WindowTest::theMainWindowIsCreatedWithoutQmlErrors()
{
    QTemporaryDir dir;
    QVERIFY(dir.isValid());

    // Hermetic: no launch-time request to GitHub from a test run.
    QVERIFY(QDir().mkpath(dir.path() + "/.nota"_L1));
    QFile settings(dir.path() + "/.nota/settings.json"_L1);
    QVERIFY(settings.open(QIODevice::WriteOnly));
    settings.write(QByteArrayLiteral(R"({"checkForUpdates": false})"));
    settings.close();

    Nota::setStartupVault(dir.path());

    QQmlApplicationEngine engine;
    QStringList problems;
    connect(&engine, &QQmlApplicationEngine::warnings, this, [&problems](const QList<QQmlError> &errors) {
        for (const QQmlError &error : errors) {
            problems.append(error.toString());
        }
    });
    KLocalization::setupLocalizedContext(&engine);

    engine.loadFromModule("org.kde.nota", u"Main"_s);

    QVERIFY2(problems.isEmpty(), qPrintable(problems.join(u'\n')));
    QVERIFY2(!engine.rootObjects().isEmpty(), "Main.qml produced no window");
}

void WindowTest::theThreeColumnsAreThereAndCanBeResized()
{
    // Test mode keeps this out of the developer's own state file.
    QStandardPaths::setTestModeEnabled(true);

    // What a restart looks like from the window's side: a width already in
    // the state file, written by a drag in some previous session.
    const int remembered = 321;
    KConfigGroup columnState(KSharedConfig::openStateConfig(), u"Columns"_s);
    columnState.writeEntry("folderColumn", remembered);
    columnState.sync();

    QTemporaryDir dir;
    QVERIFY(dir.isValid());
    QVERIFY(QDir().mkpath(dir.path() + "/.nota"_L1));
    QFile settings(dir.path() + "/.nota/settings.json"_L1);
    QVERIFY(settings.open(QIODevice::WriteOnly));
    settings.write(QByteArrayLiteral(R"({"checkForUpdates": false})"));
    settings.close();

    Nota::setStartupVault(dir.path());

    QQmlApplicationEngine engine;
    // Collected past the load as well as during it: a split view's handle is
    // built when the layout first runs, so a broken binding in it never shows
    // up in the errors loadFromModule() reports.
    QStringList problems;
    connect(&engine, &QQmlApplicationEngine::warnings, this, [&problems](const QList<QQmlError> &errors) {
        for (const QQmlError &error : errors) {
            problems.append(error.toString());
        }
    });
    KLocalization::setupLocalizedContext(&engine);
    engine.loadFromModule("org.kde.nota", u"Main"_s);
    QVERIFY(!engine.rootObjects().isEmpty());

    QObject *root = engine.rootObjects().constFirst();
    QVERIFY2(root->findChild<QQuickItem *>(u"columns"_s), "the columns must be laid out in a split view");

    auto *folders = root->findChild<QQuickItem *>(u"folderColumn"_s);
    auto *pages = root->findChild<QQuickItem *>(u"pageColumn"_s);
    QVERIFY(folders && pages);
    QVERIFY2(pages->width() > 0, "the page column must have a width");

    // The width a drag left behind is the width the window comes back with.
    QTRY_COMPARE_WITH_TIMEOUT(int(folders->width()), remembered, 3000);

    QTest::qWait(300);
    QVERIFY2(problems.isEmpty(), qPrintable(problems.join(u'\n')));
}

void WindowTest::aDraggedWidthOutlivesTheSession()
{
    QStandardPaths::setTestModeEnabled(true);

    QTemporaryDir dir;
    QVERIFY(dir.isValid());
    Nota::setStartupVault(dir.path());

    {
        Nota app;
        app.setFolderColumnWidth(275);
        app.setPageColumnWidth(190);
    }

    // A second instance is what the next launch is. The widths belong to the
    // machine, so they are not in the vault and do not travel with it.
    Nota next;
    QCOMPARE(next.folderColumnWidth(), 275);
    QCOMPARE(next.pageColumnWidth(), 190);
    QVERIFY(!QFile::exists(dir.path() + "/.nota/settings.json"_L1));
}

QTEST_MAIN(WindowTest)

#include "windowtest.moc"
