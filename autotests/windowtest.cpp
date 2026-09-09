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

#include <KLocalizedQmlContext>
#include <KLocalizedString>

#include <QDir>
#include <QFile>
#include <QQmlApplicationEngine>
#include <QQmlError>
#include <QTemporaryDir>
#include <QTest>

using namespace Qt::StringLiterals;

class WindowTest : public QObject
{
    Q_OBJECT

private Q_SLOTS:
    void theMainWindowIsCreatedWithoutQmlErrors();
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

QTEST_MAIN(WindowTest)

#include "windowtest.moc"
