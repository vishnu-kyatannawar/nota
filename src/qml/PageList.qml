// SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
// SPDX-License-Identifier: MIT

import QtQuick
import QtQuick.Layouts
import QtQuick.Controls as QQC2

import org.kde.kirigami as Kirigami
import org.kde.nota

// The pages inside the selected folder, in their own column beside the folder
// tree. A folder of two hundred workplans is a list, not a branch: giving it
// its own column is what lets it scroll without dragging the folders with it.
Item {
    id: pages

    // The dialogs live in the sidebar, which is the one place that owns them.
    signal newPageRequested(folder: string)
    signal renameRequested(path: string)
    signal trashRequested(path: string)

    Kirigami.Theme.colorSet: Kirigami.Theme.Window
    Kirigami.Theme.inherit: false

    /*! Brings the open page into view, for when it was opened from elsewhere. */
    function scrollToCurrent(): void {
        const row = Nota.pages.rowForPath(Nota.currentPath);
        if (row >= 0) {
            list.positionViewAtIndex(row, ListView.Contain);
        }
    }

    Rectangle {
        anchors.fill: parent
        color: Kirigami.Theme.backgroundColor
    }

    ColumnLayout {
        anchors.fill: parent
        spacing: 0

        RowLayout {
            Layout.fillWidth: true
            Layout.margins: Kirigami.Units.smallSpacing
            spacing: Kirigami.Units.smallSpacing

            QQC2.Button {
                objectName: "quickNewPage"
                Layout.fillWidth: true
                icon.name: "document-new"
                text: i18nc("@action:button", "New page")
                onClicked: pages.newPageRequested(Nota.pages.folder)
            }
        }

        QQC2.Label {
            Layout.fillWidth: true
            Layout.leftMargin: Kirigami.Units.smallSpacing
            Layout.rightMargin: Kirigami.Units.smallSpacing
            Layout.bottomMargin: Kirigami.Units.smallSpacing
            text: Nota.pages.folder.length > 0
                ? i18ncp("@info:status folder name, page count", "%2 · %1 page", "%2 · %1 pages",
                         Nota.pages.count, Nota.pages.folder)
                : i18ncp("@info:status page count at the vault root", "%1 page", "%1 pages", Nota.pages.count)
            elide: Text.ElideMiddle
            font: Kirigami.Theme.smallFont
            opacity: 0.7
        }

        Kirigami.Separator {
            Layout.fillWidth: true
        }

        QQC2.ScrollView {
            Layout.fillWidth: true
            Layout.fillHeight: true
            clip: true

            // Always reserve the bar rather than let it appear over the last
            // few characters of a title once the list grows past the window.
            QQC2.ScrollBar.vertical.policy: QQC2.ScrollBar.AsNeeded

            ListView {
                id: list
                objectName: "pageList"
                model: Nota.pages
                currentIndex: -1
                reuseItems: true

                delegate: QQC2.ItemDelegate {
                    id: pageDelegate

                    required property int index
                    required property string name
                    required property string path

                    width: ListView.view.width
                    highlighted: path === Nota.currentPath
                    text: name

                    onClicked: Nota.open(path)

                    TapHandler {
                        acceptedButtons: Qt.RightButton
                        onTapped: {
                            pageMenu.path = pageDelegate.path;
                            pageMenu.popup();
                        }
                    }
                }

                Kirigami.PlaceholderMessage {
                    anchors.centerIn: parent
                    width: parent.width - Kirigami.Units.gridUnit * 2
                    visible: list.count === 0
                    text: i18n("No pages here")
                    helpfulAction: Kirigami.Action {
                        text: i18nc("@action:button", "New page")
                        icon.name: "document-new"
                        onTriggered: pages.newPageRequested(Nota.pages.folder)
                    }
                }
            }
        }
    }

    QQC2.Menu {
        id: pageMenu
        property string path: ""

        QQC2.MenuItem {
            text: i18nc("@action:inmenu", "Rename…")
            icon.name: "edit-rename"
            enabled: !Nota.isDatedPage(pageMenu.path)
            onTriggered: pages.renameRequested(pageMenu.path)
        }
        QQC2.MenuItem {
            text: i18nc("@action:inmenu", "Move to trash")
            icon.name: "edit-delete"
            onTriggered: pages.trashRequested(pageMenu.path)
        }
    }

    Connections {
        target: Nota
        function onCurrentChanged(): void {
            pages.scrollToCurrent();
        }
    }
}
