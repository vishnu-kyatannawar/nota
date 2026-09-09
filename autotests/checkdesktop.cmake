# SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
# SPDX-License-Identifier: MIT
#
# Checks the generated desktop entry names an absolute binary.

if(NOT EXISTS "${FILE}")
    message(FATAL_ERROR "no desktop file at ${FILE}")
endif()

file(READ "${FILE}" contents)
if(NOT contents MATCHES "\nExec=/")
    message(FATAL_ERROR
        "Exec must be an absolute path, or the launcher resolves \"nota\" through "
        "PATH and can start a different install. Got:\n${contents}")
endif()

find_program(DESKTOP_FILE_VALIDATE desktop-file-validate)
if(DESKTOP_FILE_VALIDATE)
    execute_process(COMMAND "${DESKTOP_FILE_VALIDATE}" "${FILE}" RESULT_VARIABLE rc)
    if(NOT rc EQUAL 0)
        message(FATAL_ERROR "desktop-file-validate rejected ${FILE}")
    endif()
endif()
