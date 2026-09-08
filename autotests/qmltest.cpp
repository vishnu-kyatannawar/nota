/*
 * SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
 * SPDX-License-Identifier: MIT
 *
 * Drives the real QML against a real vault.
 *
 * The C++ suites can prove the model is right and the bytes on disk are right.
 * They cannot notice that a delegate is zero pixels tall, or that a keystroke
 * never reaches the row it was aimed at — which is exactly the class of bug
 * that makes an editor unusable while every other test stays green.
 */

#include "app.h"
#include "mdnote.h"

#include <QDir>
#include <QFile>
#include <KLocalizedQmlContext>
#include <QQmlEngine>
#include <QTemporaryDir>
#include <QtQuickTest>

using namespace Qt::StringLiterals;

class Setup : public QObject
{
    Q_OBJECT

public:
    Setup()
        : m_dir(std::make_unique<QTemporaryDir>())
    {
        // Seed a previous workplan so today rolls over with something in it:
        // an editor with no rows tests very little.
        const QDate yesterday = QDate::currentDate().addDays(-1);
        const QString path = m_dir->path() + "/Workplans/"_L1 + yesterday.toString(u"yyyy-MM-dd"_s) + ".md"_L1;
        QDir().mkpath(QFileInfo(path).absolutePath());

        QFile f(path);
        if (f.open(QIODevice::WriteOnly)) {
            f.write(QStringLiteral("---\ntype: workplan\ndate: %1\n---\n\n"
                                   "- [ ] First carried item <!--n id:AAA t:09:00-->\n"
                                   "- [ ] Second carried item <!--n id:BBB t:09:01-->\n"
                                   "- [ ] Third carried item <!--n id:CCC t:09:02-->\n")
                        .arg(yesterday.toString(u"yyyy-MM-dd"_s))
                        .toUtf8());
        }
        Nota::setStartupVault(m_dir->path());
    }

public Q_SLOTS:
    void qmlEngineAvailable(QQmlEngine *engine)
    {
        // main.cpp does this for the real app; without it every i18n() call in
        // QML yields undefined and the components fail in ways that have
        // nothing to do with what is being tested.
        KLocalization::setupLocalizedContext(engine);
    }

private:
    std::unique_ptr<QTemporaryDir> m_dir;
};

QUICK_TEST_MAIN_WITH_SETUP(nota, Setup)

#include "qmltest.moc"
