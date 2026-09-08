# SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
# SPDX-License-Identifier: MIT

include(ECMAddTests)

# The golden files the parser must round-trip byte for byte.
add_definitions(-DNOTA_TEST_DATA_DIR="${CMAKE_CURRENT_SOURCE_DIR}/data")

ecm_add_tests(
    mdnotetest.cpp
    workplantest.cpp
    itemmodeltest.cpp
    savetest.cpp
    settingstest.cpp
    vaulttest.cpp
    NAME_PREFIX "nota-"
    LINK_LIBRARIES notaqml Qt6::Test
)

# Drives the real QML. Offscreen, so it runs in CI without a display.
if(TARGET Qt6::QuickTest)
    add_executable(notaqmltest qmltest.cpp)
    target_link_libraries(notaqmltest PRIVATE notaqml Qt6::QuickTest Qt6::Qml Qt6::Quick Qt6::Widgets KF6::I18nQml)
    target_compile_definitions(notaqmltest PRIVATE
        QUICK_TEST_SOURCE_DIR="${CMAKE_CURRENT_SOURCE_DIR}/qml")
    qt_import_qml_plugins(notaqmltest)
    add_test(NAME nota-qmltest COMMAND notaqmltest)
    set_tests_properties(nota-qmltest PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen")
endif()
