# SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
# SPDX-License-Identifier: MIT

# The vault logic: no QML, no GUI, so the parser, the rollover rules and the
# settings can be tested without a window.
add_library(notacore STATIC
    mdnote.cpp
    vault.cpp
    settings.cpp
    workplan.cpp
)

target_include_directories(notacore PUBLIC ${CMAKE_CURRENT_SOURCE_DIR})
target_link_libraries(notacore PUBLIC Qt6::Core KF6::CoreAddons KF6::I18n)

# PUBLIC so everything downstream inherits them: without the domain every
# i18n() call warns that no catalogue is set, and the version is wanted by the
# sidebar as much as by --version, so it cannot live on the executable alone.
target_compile_definitions(notacore PUBLIC
    TRANSLATION_DOMAIN="nota"
    NOTA_VERSION_STRING="${PROJECT_VERSION}")
set_property(TARGET notacore PROPERTY POSITION_INDEPENDENT_CODE ON)

# The QML module lives in a library rather than in the executable, so the tests
# can link it and drive the real components. An executable's QML is compiled
# into its own resources, where no other binary can reach it — which is exactly
# why the first attempt at a QML test could not find a single type.
add_library(notaqml STATIC
    app.cpp
    itemmodel.cpp
    foldertreemodel.cpp
)

set_property(TARGET notaqml PROPERTY POSITION_INDEPENDENT_CODE ON)

target_link_libraries(notaqml PUBLIC
    notacore
    Qt6::Quick
    Qt6::QuickControls2
    KF6::ConfigCore
    KF6::ConfigGui
    KF6::ItemModels
    KF6::SyntaxHighlighting
)

ecm_add_qml_module(notaqml
    URI org.kde.nota
    VERSION 1.0
    GENERATE_PLUGIN_SOURCE
    IMPORTS
        org.kde.kirigami
        org.kde.kitemmodels
)

ecm_target_qml_sources(notaqml SOURCES
    qml/Main.qml
    qml/Sidebar.qml
    qml/SidebarButton.qml
    qml/NotePage.qml
    qml/DayHeader.qml
    qml/ItemList.qml
    qml/ItemRow.qml
    qml/NotesEditor.qml
    qml/RawEditor.qml
)

ecm_finalize_qml_module(notaqml)

add_executable(nota main.cpp)

target_link_libraries(nota PRIVATE
    notaqml
    Qt6::Widgets
    KF6::CoreAddons
    KF6::I18n
    KF6::I18nQml
    KirigamiApp
)

# A statically linked QML module ships its types in a plugin, and a static
# plugin has to be pulled in explicitly or the engine reports the module as
# missing at import time.
qt_import_qml_plugins(nota)

install(TARGETS nota ${KDE_INSTALL_TARGETS_DEFAULT_ARGS})
