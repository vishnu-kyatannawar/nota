// SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
// SPDX-License-Identifier: MIT

import QtQuick
import QtQuick.Controls as QQC2

import org.kde.kirigami as Kirigami
import org.kde.nota

// The free-form markdown under the items. Plain text on purpose: Qt's markdown
// writer normalises what it reads, which would break the round trip.
QQC2.TextArea {
    id: notes

    textFormat: TextEdit.PlainText
    wrapMode: TextEdit.Wrap
    background: null
    placeholderText: i18n("Notes")
    font: Kirigami.Theme.fixedWidthFont

    Component.onCompleted: text = Nota.body
    onTextChanged: pushTimer.restart()
    onActiveFocusChanged: if (!activeFocus) {
        Nota.setBody(text);
    }

    Timer {
        id: pushTimer
        interval: 150
        onTriggered: Nota.setBody(notes.text)
    }

    Connections {
        target: Nota
        function onCurrentChanged(): void {
            if (!notes.activeFocus && notes.text !== Nota.body) {
                notes.text = Nota.body;
            }
        }
    }
}
