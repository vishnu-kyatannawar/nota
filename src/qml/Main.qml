// SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
// SPDX-License-Identifier: MIT

import QtQuick
import QtQuick.Layouts
import QtQuick.Controls as QQC2

import org.kde.kirigami as Kirigami
import org.kde.nota

Kirigami.ApplicationWindow {
    id: root

    title: Nota.title.length > 0 ? i18nc("@title:window", "%1 — Nota", Nota.title) : i18nc("@title:window", "Nota")

    minimumWidth: Kirigami.Units.gridUnit * 34
    minimumHeight: Kirigami.Units.gridUnit * 22
    width: Kirigami.Units.gridUnit * 64
    height: Kirigami.Units.gridUnit * 42

    // The two panes are laid out here rather than pushed onto the page stack:
    // this is a desktop window with a sidebar, not a drill-down.
    pageStack.globalToolBar.style: Kirigami.ApplicationHeaderStyle.None

    // Rollover runs on launch, at midnight and whenever the window regains
    // focus — the third is what catches a machine that was asleep at midnight.
    onActiveChanged: if (active) {
        Nota.openToday();
    }

    pageStack.initialPage: Kirigami.Page {
        padding: 0

        ColumnLayout {
            anchors.fill: parent
            spacing: 0

            Kirigami.InlineMessage {
                Layout.fillWidth: true
                type: Kirigami.MessageType.Error
                text: Nota.errorMessage
                visible: Nota.errorMessage.length > 0
                showCloseButton: true
                onVisibleChanged: if (!visible) {
                    Nota.clearError();
                }
            }

            RowLayout {
                Layout.fillWidth: true
                Layout.fillHeight: true
                spacing: 0

                Sidebar {
                    Layout.preferredWidth: Kirigami.Units.gridUnit * 15
                    Layout.fillHeight: true
                }

                Kirigami.Separator {
                    Layout.fillHeight: true
                    Layout.preferredWidth: 1
                }

                NotePage {
                    Layout.fillWidth: true
                    Layout.fillHeight: true
                }
            }
        }
    }
}
