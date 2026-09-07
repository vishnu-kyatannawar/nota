// SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
// SPDX-License-Identifier: MIT

import QtQuick
import QtQuick.Layouts
import QtQuick.Controls as QQC2

import org.kde.kirigami as Kirigami
import org.kde.nota

// The open page: its header, its action items and the prose under them.
// Read only for now — the editing model lands next.
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
        contentWidth: availableWidth
        clip: true

        ColumnLayout {
            width: page.width
            spacing: Kirigami.Units.largeSpacing

            // --- header ------------------------------------------------------
            ColumnLayout {
                Layout.fillWidth: true
                Layout.margins: Kirigami.Units.largeSpacing
                Layout.bottomMargin: 0
                spacing: Kirigami.Units.smallSpacing

                RowLayout {
                    Layout.fillWidth: true
                    spacing: Kirigami.Units.largeSpacing

                    Kirigami.Heading {
                        text: Nota.title
                        level: 1
                        elide: Text.ElideRight
                    }

                    QQC2.Label {
                        visible: Nota.isWorkplan && Nota.dayType.length > 0
                        text: Nota.dayType
                        font: Kirigami.Theme.smallFont
                        opacity: 0.7
                        leftPadding: Kirigami.Units.smallSpacing
                        rightPadding: Kirigami.Units.smallSpacing
                        background: Rectangle {
                            radius: Kirigami.Units.cornerRadius
                            color: "transparent"
                            border.color: Kirigami.Theme.separatorColor
                            border.width: 1
                        }
                    }

                    Item {
                        Layout.fillWidth: true
                    }

                    QQC2.Label {
                        text: i18n("%1 open · %2 done", Nota.openCount, Nota.doneCount)
                        font: Kirigami.Theme.smallFont
                        opacity: 0.7
                    }
                }

                QQC2.Label {
                    text: Nota.subtitle
                    font: Kirigami.Theme.smallFont
                    opacity: 0.55
                    elide: Text.ElideMiddle
                    Layout.fillWidth: true
                }
            }

            // --- items -------------------------------------------------------
            Repeater {
                model: Nota.items

                delegate: ItemRow {
                    Layout.fillWidth: true
                    Layout.leftMargin: Kirigami.Units.largeSpacing
                    Layout.rightMargin: Kirigami.Units.largeSpacing
                }
            }

            // --- the prose under the items -----------------------------------
            QQC2.TextArea {
                Layout.fillWidth: true
                Layout.margins: Kirigami.Units.largeSpacing
                visible: Nota.body.length > 0
                // Plain text on purpose: Qt's markdown writer normalises what it
                // reads, which would break the byte-for-byte round trip.
                textFormat: TextEdit.PlainText
                text: Nota.body
                readOnly: true
                wrapMode: TextEdit.Wrap
                background: null
            }

            Item {
                Layout.preferredHeight: Kirigami.Units.gridUnit
            }
        }
    }
}
