# SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
# SPDX-License-Identifier: MIT

include(ECMAddTests)

# The golden files the parser must round-trip byte for byte.
add_definitions(-DNOTA_TEST_DATA_DIR="${CMAKE_CURRENT_SOURCE_DIR}/data")

ecm_add_tests(
    mdnotetest.cpp
    workplantest.cpp
    itemmodeltest.cpp
    settingstest.cpp
    vaulttest.cpp
    NAME_PREFIX "nota-"
    LINK_LIBRARIES notacore Qt6::Test
)
