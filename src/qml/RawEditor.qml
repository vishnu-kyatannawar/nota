// SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
// SPDX-License-Identifier: MIT

import QtQuick
import QtQuick.Layouts
import QtQuick.Controls as QQC2

import org.kde.kirigami as Kirigami
import org.kde.syntaxhighlighting as SH
import org.kde.nota

// The whole file as text. The escape hatch for anything the structured editor
// cannot express — and cheap to offer, because the format round-trips.
ColumnLayout {
    id: rawEditor

    signal closed

    function load(): void {
        area.text = Nota.raw();
        area.forceActiveFocus();
    }

    RowLayout {
        Layout.fillWidth: true
        Layout.margins: Kirigami.Units.largeSpacing
        spacing: Kirigami.Units.smallSpacing

        Kirigami.Heading {
            text: i18nc("@title", "Raw markdown")
            level: 3
            Layout.fillWidth: true
        }

        QQC2.Button {
            text: i18nc("@action:button", "Save")
            icon.name: "document-save"
            onClicked: if (Nota.saveRaw(area.text)) {
                rawEditor.closed();
            }
        }

        QQC2.Button {
            text: i18nc("@action:button", "Cancel")
            icon.name: "dialog-cancel"
            onClicked: rawEditor.closed()
        }
    }

    QQC2.TextArea {
        id: area
        Layout.fillWidth: true
        Layout.leftMargin: Kirigami.Units.largeSpacing
        Layout.rightMargin: Kirigami.Units.largeSpacing
        Layout.bottomMargin: Kirigami.Units.largeSpacing

        textFormat: TextEdit.PlainText
        wrapMode: TextEdit.NoWrap
        font: Kirigami.Theme.fixedWidthFont

        SH.SyntaxHighlighter {
            textEdit: area
            definition: SH.Repository.definitionForName("Markdown")
            theme: SH.Repository.defaultTheme(Kirigami.Theme.backgroundColor.hslLightness > 0.5
                ? SH.Repository.LightTheme
                : SH.Repository.DarkTheme)
        }

        Keys.onPressed: event => {
            if (event.key === Qt.Key_E && (event.modifiers & Qt.ControlModifier)) {
                rawEditor.closed();
                event.accepted = true;
            }
        }
    }
}
