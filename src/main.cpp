/*
 * SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
 * SPDX-License-Identifier: MIT
 */

#include "app.h"

#include <KAboutData>
#include <KLocalizedQmlContext>
#include <KLocalizedString>
#include <KirigamiAppDefaults>

#include <QApplication>
#include <QCommandLineParser>
#include <QQmlApplicationEngine>

using namespace Qt::StringLiterals;

int main(int argc, char *argv[])
{
    // QApplication rather than QGuiApplication: qqc2-desktop-style is
    // QStyle-based, so it needs the widgets application to look native.
    QApplication app(argc, argv);

    // Style, Breeze icons, colour scheme, font size, logging and the crash
    // handler, all before any window exists — setting the style afterwards is
    // ignored with only a warning.
    KirigamiAppDefaults::apply(&app);

    KLocalizedString::setApplicationDomain(QByteArrayLiteral("nota"));

    KAboutData about(u"nota"_s,
                     i18nc("@title", "Nota"),
                     QStringLiteral(NOTA_VERSION_STRING),
                     i18n("Daily workplans and notes, stored as plain markdown"),
                     KAboutLicense::MIT,
                     i18n("© 2026 Vishnu Kyatannawar"));
    about.addAuthor(i18nc("@info:credit", "Vishnu Kyatannawar"), {}, u"vishnukyatannawar@gmail.com"_s);
    about.setHomepage(u"https://vishnu-kyatannawar.github.io/nota/"_s);
    about.setBugAddress(QByteArrayLiteral("https://github.com/vishnu-kyatannawar/nota/issues"));
    // On Wayland the window's app id comes from the desktop file name, so this
    // has to match the installed .desktop exactly.
    about.setDesktopFileName(u"io.github.vishnu_kyatannawar.Nota"_s);
    KAboutData::setApplicationData(about);

    QCommandLineParser parser;
    parser.addPositionalArgument(u"vault"_s, i18n("Path to a vault directory"), u"[vault]"_s);
    about.setupCommandLine(&parser);
    parser.process(app);
    about.processCommandLine(&parser);

    if (!parser.positionalArguments().isEmpty()) {
        Nota::setStartupVault(parser.positionalArguments().constFirst());
    }

    QQmlApplicationEngine engine;
    // Must come before the first component is built, or every i18n() call in
    // QML quietly yields undefined and the labels come out blank. It only picks
    // up the catalogue because TRANSLATION_DOMAIN is defined on the target.
    KLocalization::setupLocalizedContext(&engine);

    QObject::connect(
        &engine,
        &QQmlApplicationEngine::objectCreationFailed,
        &app,
        [] {
            QCoreApplication::exit(1);
        },
        Qt::QueuedConnection);

    engine.loadFromModule("org.kde.nota", u"Main"_s);
    if (engine.rootObjects().isEmpty()) {
        return 1;
    }
    return app.exec();
}
