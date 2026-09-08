# SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
# SPDX-License-Identifier: MIT

# The vault logic is a plain library with no QML and no GUI in it, so the
# parser, the rollover rules and the settings can be tested without a window.
add_library(notacore STATIC
    mdnote.cpp
    vault.cpp
    settings.cpp
    workplan.cpp
    itemmodel.cpp
    foldertreemodel.cpp
)

target_include_directories(notacore PUBLIC ${CMAKE_CURRENT_SOURCE_DIR})
target_link_libraries(notacore PUBLIC Qt6::Core Qt6::Qml KF6::CoreAddons KF6::I18n)

# PUBLIC so the executable and the tests inherit it: without the definition
# every i18n() call warns that no catalogue is set.
target_compile_definitions(notacore PUBLIC TRANSLATION_DOMAIN="nota")
set_property(TARGET notacore PROPERTY POSITION_INDEPENDENT_CODE ON)

add_executable(nota main.cpp)

target_sources(nota PRIVATE
    app.cpp
)

# The backing target is an executable, so Qt forces NO_PLUGIN and the QML is
# compiled into the binary's own resources. That is also why there is no
# ecm_finalize_qml_module() call: it early-returns when there is no plugin.
ecm_add_qml_module(nota
    URI org.kde.nota
    VERSION 1.0
    IMPORTS
        org.kde.kirigami
        org.kde.kitemmodels
)

ecm_target_qml_sources(nota SOURCES
    qml/Main.qml
    qml/Sidebar.qml
    qml/NotePage.qml
    qml/ItemRow.qml
)

target_compile_definitions(nota PRIVATE NOTA_VERSION_STRING="${PROJECT_VERSION}")

target_link_libraries(nota PRIVATE
    notacore
    Qt6::Quick
    Qt6::QuickControls2
    Qt6::Widgets
    KF6::CoreAddons
    KF6::I18n
    KF6::I18nQml
    KF6::ConfigCore
    KF6::ConfigGui
    KF6::ItemModels
    KF6::SyntaxHighlighting
    KirigamiApp
)

install(TARGETS nota ${KDE_INSTALL_TARGETS_DEFAULT_ARGS})
