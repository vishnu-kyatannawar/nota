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

    // Ctrl+E swaps the whole page to text and back.
    property bool rawMode: false

    Kirigami.Theme.colorSet: Kirigami.Theme.View
    Kirigami.Theme.inherit: false

    function toggleRaw(): void {
        if (Nota.currentPath.length === 0) {
            return;
        }
        if (!rawMode) {
            Nota.flush();
        }
        rawMode = !rawMode;
        if (rawMode) {
            (rawLoader.item as RawEditor).load();
        }
    }

    Rectangle {
        anchors.fill: parent
        color: Kirigami.Theme.backgroundColor
    }

    // The sidebar owns creating things, because it also has to open the tree
    // down to whatever was made. This only asks.
    signal newPageRequested(folder: string)
    signal newFolderRequested(parent: string)

    Kirigami.PlaceholderMessage {
        anchors.centerIn: parent
        width: parent.width - Kirigami.Units.gridUnit * 4
        visible: Nota.currentPath.length === 0 && Nota.currentFolder.length === 0
        icon.name: "text-markdown"
        text: i18n("Pick a page")
        explanation: i18n("Choose something in the sidebar, or open today's workplan.")
    }

    // A folder holds no note to show. Offering to fill it is the useful thing
    // to do with that space — an empty one used to report that a directory
    // could not be read as a file, which is true and no help to anybody.
    Kirigami.PlaceholderMessage {
        anchors.centerIn: parent
        width: parent.width - Kirigami.Units.gridUnit * 4
        objectName: "folderPlaceholder"
        visible: Nota.currentFolder.length > 0
        icon.name: "folder-symbolic"
        text: i18n("Nothing here yet")
        explanation: Nota.currentFolder

        RowLayout {
            Layout.alignment: Qt.AlignHCenter
            spacing: Kirigami.Units.smallSpacing

            QQC2.Button {
                objectName: "folderNewPage"
                text: i18nc("@action:button", "New page")
                icon.name: "document-new"
                onClicked: page.newPageRequested(Nota.currentFolder)
            }

            QQC2.Button {
                objectName: "folderNewFolder"
                text: i18nc("@action:button", "New folder")
                icon.name: "folder-new"
                onClicked: page.newFolderRequested(Nota.currentFolder)
            }
        }
    }

    Loader {
        id: rawLoader
        anchors.fill: parent
        active: page.rawMode
        visible: active
        sourceComponent: RawEditor {
            onClosed: page.rawMode = false
        }
    }

    QQC2.ScrollView {
        anchors.fill: parent
        visible: Nota.currentPath.length > 0 && !page.rawMode
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

                RowLayout {
                    Layout.fillWidth: true
                    Layout.leftMargin: Kirigami.Units.largeSpacing
                    Layout.rightMargin: Kirigami.Units.largeSpacing
                    Layout.topMargin: Kirigami.Units.smallSpacing
                    spacing: Kirigami.Units.smallSpacing

                    QQC2.Button {
                        text: i18nc("@action:button", "Add item")
                        icon.name: "list-add"
                        flat: true
                        onClicked: items.focusRow(items.model.insertItem(items.count - 1, 0), 0)
                    }

                    QQC2.TextField {
                        id: repeatField
                        visible: Nota.isWorkplan
                        Layout.fillWidth: true
                        placeholderText: i18nc("@info:placeholder", "something to do every day…")
                        // Adding one seeds it into today as well as tomorrow,
                        // which is plainly what someone typing it at 09:00 meant.
                        onAccepted: {
                            if (text.trim().length > 0 && Nota.addRepeating(text)) {
                                text = "";
                            }
                        }
                        Keys.onEscapePressed: text = ""
                    }
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
