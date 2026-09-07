# SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
# SPDX-License-Identifier: MIT

cmake_minimum_required(VERSION 3.24)

project(nota VERSION 0.2.0 LANGUAGES CXX)

# Kirigami 6.29 itself requires Qt >= 6.9, so claiming less would be a lie.
set(QT_MIN_VERSION 6.9.0)
set(KF_MIN_VERSION 6.10.0)
set(KIRIGAMI_ADDONS_MIN_VERSION 1.11.0) # KirigamiAppDefaults::apply()

find_package(ECM ${KF_MIN_VERSION} REQUIRED NO_MODULE)
set(CMAKE_MODULE_PATH ${ECM_MODULE_PATH})

include(KDEInstallDirs)
include(KDECMakeSettings)
include(KDECompilerSettings NO_POLICY_SCOPE)
include(ECMQmlModule)
include(ECMFindQmlModule)
include(ECMAddTests)
include(ECMDeprecationSettings)
include(ECMInstallIcons)
include(FeatureSummary)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

find_package(Qt6 ${QT_MIN_VERSION} REQUIRED COMPONENTS
    Core Gui Qml Quick QuickControls2 Widgets Test)

find_package(KF6 ${KF_MIN_VERSION} REQUIRED COMPONENTS
    CoreAddons          # KAboutData, KDirWatch
    I18n                # KLocalizedString, KF6::I18nQml
    Config              # machine-local state: window geometry, last vault
    ItemModels          # KDescendantsProxyModel
    ColorScheme
    IconThemes
    Crash
    SyntaxHighlighting
    Kirigami
)
find_package(KF6KirigamiAddons ${KIRIGAMI_ADDONS_MIN_VERSION} REQUIRED)

# Runtime-only QML dependencies. Not linked; declared so a packager who forgets
# one gets a configure-time error rather than the user getting a blank window.
ecm_find_qmlmodule(org.kde.kirigami REQUIRED)
ecm_find_qmlmodule(org.kde.kitemmodels REQUIRED)
ecm_find_qmlmodule(org.kde.coreaddons REQUIRED)
ecm_find_qmlmodule(org.kde.syntaxhighlighting REQUIRED)
ecm_find_qmlmodule(org.kde.kirigamiaddons.formcard REQUIRED)
ecm_find_qmlmodule(org.kde.kirigamiaddons.delegates REQUIRED)
ecm_find_qmlmodule(org.kde.kirigamiaddons.components REQUIRED)
ecm_find_qmlmodule(org.kde.desktop REQUIRED) # qqc2-desktop-style

ecm_set_disabled_deprecation_versions(QT 6.9 KF 6.10)

add_subdirectory(src)

if(BUILD_TESTING)
    add_subdirectory(autotests)
endif()

install(PROGRAMS io.github.vishnu_kyatannawar.Nota.desktop
        DESTINATION ${KDE_INSTALL_APPDIR})
install(FILES io.github.vishnu_kyatannawar.Nota.metainfo.xml
        DESTINATION ${KDE_INSTALL_METAINFODIR})
ecm_install_icons(ICONS
    icons/sc-apps-io.github.vishnu_kyatannawar.Nota.svg
    DESTINATION ${KDE_INSTALL_ICONDIR})

feature_summary(WHAT ALL FATAL_ON_MISSING_REQUIRED_PACKAGES)
