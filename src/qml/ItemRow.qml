// SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
// SPDX-License-Identifier: MIT

import QtQuick
import QtQuick.Layouts
import QtQuick.Controls as QQC2

import org.kde.kirigami as Kirigami

// One action item, or a group heading between items.
ColumnLayout {
    id: row

    required property string text
    required property bool done
    required property int depth
    required property bool isHeading
    required property int headingLevel
    required property var labels
    required property string durationText
    required property string createdAt
    required property string doneAt
    required property string from
    required property int carried
    required property string recurring
    required property string body
    required property bool hasBody

    spacing: Kirigami.Units.smallSpacing
    // Two spaces per level in the file; one grid unit per level on screen.
    Layout.leftMargin: Kirigami.Units.gridUnit * row.depth

    Kirigami.Heading {
        visible: row.isHeading
        text: row.text
        level: Math.min(Math.max(row.headingLevel, 2), 5)
        Layout.fillWidth: true
        Layout.topMargin: Kirigami.Units.largeSpacing
    }

    RowLayout {
        visible: !row.isHeading
        Layout.fillWidth: true
        spacing: Kirigami.Units.smallSpacing

        QQC2.CheckBox {
            checked: row.done
            // Ticking arrives with the editing model; showing state is enough
            // to tell whether the port reads a real vault correctly.
            enabled: false
            opacity: 1
            Layout.alignment: Qt.AlignTop
        }

        ColumnLayout {
            Layout.fillWidth: true
            spacing: 2

            RowLayout {
                Layout.fillWidth: true
                spacing: Kirigami.Units.smallSpacing

                QQC2.Label {
                    text: row.text
                    wrapMode: Text.Wrap
                    Layout.fillWidth: true
                    opacity: row.done ? 0.55 : 1
                    font.strikeout: row.done
                }

                Kirigami.Icon {
                    visible: row.recurring.length > 0
                    source: "view-refresh-symbolic"
                    implicitWidth: Kirigami.Units.iconSizes.small
                    implicitHeight: Kirigami.Units.iconSizes.small
                    opacity: 0.6

                    QQC2.ToolTip.text: i18n("Repeats every day")
                    QQC2.ToolTip.visible: hoverHandler.hovered
                    QQC2.ToolTip.delay: Kirigami.Units.toolTipDelay
                    HoverHandler {
                        id: hoverHandler
                    }
                }

                QQC2.Label {
                    visible: row.carried > 0
                    text: i18np("carried %1 day", "carried %1 days", row.carried)
                    font: Kirigami.Theme.smallFont
                    opacity: 0.6
                }

                QQC2.Label {
                    visible: row.durationText.length > 0
                    text: row.durationText
                    font: Kirigami.Theme.fixedWidthFont
                    opacity: 0.6
                }

                QQC2.Label {
                    visible: row.createdAt.length > 0
                    text: row.doneAt.length > 0
                        ? i18nc("@info item timestamps", "added %1 · done %2", row.createdAt, row.doneAt)
                        : i18nc("@info item timestamp", "added %1", row.createdAt)
                    font: Kirigami.Theme.smallFont
                    opacity: 0.45
                }
            }

            QQC2.Label {
                visible: row.hasBody
                text: row.body
                wrapMode: Text.Wrap
                font: Kirigami.Theme.fixedWidthFont
                opacity: 0.75
                Layout.fillWidth: true
                Layout.leftMargin: Kirigami.Units.smallSpacing
            }
        }
    }
}
