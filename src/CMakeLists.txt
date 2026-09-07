# SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
# SPDX-License-Identifier: MIT

# The vault logic is a plain library with no QML and no GUI in it, so the
# parser, the rollover rules and the settings can be tested without a window.
add_library(notacore STATIC
    mdnote.cpp
    vault.cpp
    settings.cpp
    workplan.cpp
)

target_include_directories(notacore PUBLIC ${CMAKE_CURRENT_SOURCE_DIR})
target_link_libraries(notacore PUBLIC Qt6::Core KF6::CoreAddons KF6::I18n)

# PUBLIC so the executable and the tests inherit it: without the definition
# every i18n() call warns that no catalogue is set.
target_compile_definitions(notacore PUBLIC TRANSLATION_DOMAIN="nota")
set_property(TARGET notacore PROPERTY POSITION_INDEPENDENT_CODE ON)
