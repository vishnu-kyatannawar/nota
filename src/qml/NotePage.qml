// SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
// SPDX-License-Identifier: MIT

import QtQuick
import QtQuick.Layouts
import QtQuick.Controls as QQC2

import org.kde.kirigami as Kirigami
import org.kde.nota

// The open page: its header, its action items and the prose under them.
Item {
    id: page

    Kirigami.Theme.colorSet: Kirigami.Theme.View
    Kirigami.Theme.inherit: false

    Rectangle {
        anchors.fill: parent
        color: Kirigami.Theme.backgroundColor
    }

    Kirigami.PlaceholderMessage {
        anchors.centerIn: parent
        width: parent.width - Kirigami.Units.gridUnit * 4
        visible: Nota.currentPath.length === 0
        icon.name: "text-markdown"
        text: i18n("Pick a page")
        explanation: i18n("Choose something in the sidebar, or open today's workplan.")
    }

    QQC2.ScrollView {
        anchors.fill: parent
        visible: Nota.currentPath.length > 0
        clip: true

        // One scrolling surface for the whole page: the header and the notes
        // ride with the items rather than fighting them for the wheel.
        ItemList {
            id: items

            header: DayHeader {
                width: items.width
            }

            footer: ColumnLayout {
                width: items.width
                spacing: Kirigami.Units.smallSpacing

                QQC2.Button {
                    text: i18nc("@action:button", "Add item")
                    icon.name: "list-add"
                    flat: true
                    Layout.leftMargin: Kirigami.Units.largeSpacing
                    Layout.topMargin: Kirigami.Units.smallSpacing
                    onClicked: items.focusRow(items.model.insertItem(items.count - 1, 0), 0)
                }

                Kirigami.Separator {
                    Layout.fillWidth: true
                    Layout.margins: Kirigami.Units.largeSpacing
                    visible: notes.visible
                }

                NotesEditor {
                    id: notes
                    Layout.fillWidth: true
                    Layout.leftMargin: Kirigami.Units.largeSpacing
                    Layout.rightMargin: Kirigami.Units.largeSpacing
                    Layout.bottomMargin: Kirigami.Units.gridUnit
                }
            }
        }
    }
}
