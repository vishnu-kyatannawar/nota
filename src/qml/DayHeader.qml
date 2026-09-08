// SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
// SPDX-License-Identifier: MIT

import QtQuick
import QtQuick.Layouts
import QtQuick.Controls as QQC2

import org.kde.kirigami as Kirigami
import org.kde.nota

// The page header: what this page is, and the two numbers a workplan carries.
ColumnLayout {
    id: header

    spacing: Kirigami.Units.smallSpacing

    RowLayout {
        Layout.fillWidth: true
        Layout.margins: Kirigami.Units.largeSpacing
        Layout.bottomMargin: 0
        spacing: Kirigami.Units.largeSpacing

        Kirigami.Heading {
            text: Nota.title
            level: 1
            elide: Text.ElideRight
        }

        QQC2.ComboBox {
            visible: Nota.isWorkplan
            model: ["work", "weekend", "leave", "holiday"]
            currentIndex: Math.max(0, model.indexOf(Nota.dayType))
            onActivated: Nota.setDayType(currentValue)
            implicitWidth: Kirigami.Units.gridUnit * 7
        }

        Item {
            Layout.fillWidth: true
        }

        QQC2.Label {
            text: i18n("%1 open · %2 done", Nota.openCount, Nota.doneCount)
            font: Kirigami.Theme.smallFont
            opacity: 0.7
        }

        QQC2.Label {
            text: Nota.dirty ? i18nc("@info save state", "Saving…") : i18nc("@info save state", "Saved")
            font: Kirigami.Theme.smallFont
            opacity: Nota.dirty ? 0.7 : 0.35
        }
    }

    RowLayout {
        Layout.fillWidth: true
        Layout.leftMargin: Kirigami.Units.largeSpacing
        Layout.rightMargin: Kirigami.Units.largeSpacing
        spacing: Kirigami.Units.smallSpacing

        QQC2.Label {
            text: Nota.subtitle
            font: Kirigami.Theme.smallFont
            opacity: 0.55
            elide: Text.ElideMiddle
            Layout.fillWidth: true
        }

        QQC2.Label {
            visible: Nota.isWorkplan
            text: i18nc("@label:textbox hours worked", "Hours")
            font: Kirigami.Theme.smallFont
            opacity: 0.55
        }

        QQC2.TextField {
            id: hours
            visible: Nota.isWorkplan
            text: Nota.hours
            // Hours are frontmatter, so setting them never renames the file.
            inputMask: "99:99"
            implicitWidth: Kirigami.Units.gridUnit * 3.5
            horizontalAlignment: TextInput.AlignHCenter
            font: Kirigami.Theme.fixedWidthFont

            onEditingFinished: Nota.setHours(text)
            Keys.onEscapePressed: {
                text = Nota.hours;
                focus = false;
            }

            Connections {
                target: Nota
                function onCurrentChanged(): void {
                    if (!hours.activeFocus) {
                        hours.text = Nota.hours;
                    }
                }
            }
        }
    }
}
