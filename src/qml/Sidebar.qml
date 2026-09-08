// SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
// SPDX-License-Identifier: MIT

import QtQuick
import QtQuick.Layouts
import QtQuick.Controls as QQC2

import org.kde.kirigami as Kirigami
import org.kde.kitemmodels as KItemModels
import org.kde.nota

// The sidebar tree is the directory tree. The model is a real tree; flattening
// it here rather than in C++ is what buys the expand and collapse for free.
Item {
    id: sidebar

    Kirigami.Theme.colorSet: Kirigami.Theme.Window
    Kirigami.Theme.inherit: false

    // Where a new page or folder goes: inside the selected folder, beside the
    // selected page, or at the root when nothing is chosen.
    property string contextPath: ""
    property bool contextIsFolder: false

    function targetFolder(): string {
        if (contextPath.length === 0) {
            return "";
        }
        if (contextIsFolder) {
            return contextPath;
        }
        return contextPath.includes("/") ? contextPath.substring(0, contextPath.lastIndexOf("/")) : "";
    }

    Rectangle {
        anchors.fill: parent
        color: Kirigami.Theme.backgroundColor
    }

    KItemModels.KDescendantsProxyModel {
        id: flatTree
        model: Nota.folderTree
        expandsByDefault: false
    }

    ColumnLayout {
        anchors.fill: parent
        spacing: 0

        RowLayout {
            Layout.fillWidth: true
            Layout.margins: Kirigami.Units.smallSpacing
            spacing: 0

            Kirigami.Icon {
                source: "io.github.vishnu_kyatannawar.Nota"
                fallback: "text-markdown"
                implicitWidth: Kirigami.Units.iconSizes.smallMedium
                implicitHeight: Kirigami.Units.iconSizes.smallMedium
            }

            Kirigami.Heading {
                text: i18nc("@title The application name", "Nota")
                level: 4
                Layout.fillWidth: true
                Layout.leftMargin: Kirigami.Units.smallSpacing
            }

            SidebarButton {
                icon.name: "go-jump-today"
                text: i18nc("@action:button", "Today's workplan")
                onClicked: Nota.openToday()
            }

            SidebarButton {
                icon.name: "document-new"
                text: i18nc("@action:button", "New page")
                onClicked: Nota.createNote(sidebar.targetFolder())
            }

            SidebarButton {
                icon.name: "folder-new"
                text: i18nc("@action:button", "New folder")
                onClicked: {
                    folderPrompt.parentFolder = sidebar.targetFolder();
                    folderPrompt.open();
                }
            }
        }

        Kirigami.Separator {
            Layout.fillWidth: true
        }

        QQC2.ScrollView {
            Layout.fillWidth: true
            Layout.fillHeight: true
            clip: true

            ListView {
                id: treeView
                model: flatTree
                currentIndex: -1

                delegate: QQC2.ItemDelegate {
                    id: treeDelegate

                    required property int index
                    required property string name
                    required property string path
                    required property bool isFolder
                    required property string iconName
                    required property int kDescendantLevel
                    required property bool kDescendantExpandable
                    required property bool kDescendantExpanded

                    width: ListView.view.width
                    highlighted: (!isFolder && path === Nota.currentPath) || path === sidebar.contextPath

                    onClicked: {
                        sidebar.contextPath = path;
                        sidebar.contextIsFolder = isFolder;
                        if (kDescendantExpandable) {
                            flatTree.toggleChildren(index);
                        } else {
                            Nota.open(path);
                        }
                    }

                    TapHandler {
                        acceptedButtons: Qt.RightButton
                        onTapped: {
                            sidebar.contextPath = treeDelegate.path;
                            sidebar.contextIsFolder = treeDelegate.isFolder;
                            rowMenu.reservedPath = Nota.isReserved(treeDelegate.path);
                            rowMenu.popup();
                        }
                    }

                    contentItem: RowLayout {
                        spacing: Kirigami.Units.smallSpacing

                        Item {
                            // One grid unit of indent per level of nesting.
                            implicitWidth: Kirigami.Units.gridUnit * (treeDelegate.kDescendantLevel - 1)
                            implicitHeight: 1
                        }

                        Kirigami.Icon {
                            source: treeDelegate.kDescendantExpandable
                                ? (treeDelegate.kDescendantExpanded ? "collapse-symbolic" : "expand-symbolic")
                                : treeDelegate.iconName
                            implicitWidth: Kirigami.Units.iconSizes.small
                            implicitHeight: Kirigami.Units.iconSizes.small
                        }

                        QQC2.Label {
                            text: treeDelegate.name
                            elide: Text.ElideRight
                            Layout.fillWidth: true
                        }
                    }
                }

                Kirigami.PlaceholderMessage {
                    anchors.centerIn: parent
                    width: parent.width - Kirigami.Units.gridUnit * 2
                    visible: treeView.count === 0
                    text: i18n("This vault is empty")
                    explanation: Nota.vaultPath
                }

                TapHandler {
                    acceptedButtons: Qt.RightButton
                    onTapped: {
                        sidebar.contextPath = "";
                        sidebar.contextIsFolder = true;
                        rowMenu.reservedPath = false;
                        rowMenu.popup();
                    }
                }
            }
        }

        Kirigami.Separator {
            Layout.fillWidth: true
        }

        QQC2.Label {
            Layout.fillWidth: true
            Layout.margins: Kirigami.Units.smallSpacing
            text: Nota.vaultPath
            elide: Text.ElideMiddle
            font: Kirigami.Theme.smallFont
            opacity: 0.7
        }
    }

    QQC2.Menu {
        id: rowMenu

        // The workplan folder is reserved: renaming or deleting it would orphan
        // every dated note under it.
        property bool reservedPath: false

        QQC2.MenuItem {
            text: i18nc("@action:inmenu", "New page here")
            icon.name: "document-new"
            onTriggered: Nota.createNote(sidebar.targetFolder())
        }
        QQC2.MenuItem {
            text: i18nc("@action:inmenu", "New folder here")
            icon.name: "folder-new"
            onTriggered: {
                folderPrompt.parentFolder = sidebar.targetFolder();
                folderPrompt.open();
            }
        }
        QQC2.MenuSeparator {}
        QQC2.MenuItem {
            text: i18nc("@action:inmenu", "Rename…")
            icon.name: "edit-rename"
            enabled: sidebar.contextPath.length > 0 && !rowMenu.reservedPath
            onTriggered: {
                renamePrompt.path = sidebar.contextPath;
                renamePrompt.open();
            }
        }
        QQC2.MenuItem {
            text: i18nc("@action:inmenu", "Move to trash")
            icon.name: "edit-delete"
            enabled: sidebar.contextPath.length > 0 && !rowMenu.reservedPath
            onTriggered: {
                deletePrompt.path = sidebar.contextPath;
                deletePrompt.open();
            }
        }
    }

    Kirigami.PromptDialog {
        id: folderPrompt
        property string parentFolder: ""

        title: i18nc("@title:dialog", "New folder")
        standardButtons: Kirigami.Dialog.Ok | Kirigami.Dialog.Cancel

        QQC2.TextField {
            id: folderName
            placeholderText: i18nc("@info:placeholder", "Folder name")
            onAccepted: folderPrompt.accept()
        }

        onOpened: {
            folderName.text = "";
            folderName.forceActiveFocus();
        }
        onAccepted: Nota.createFolder(parentFolder, folderName.text)
    }

    Kirigami.PromptDialog {
        id: renamePrompt
        property string path: ""

        title: i18nc("@title:dialog", "Rename")
        standardButtons: Kirigami.Dialog.Ok | Kirigami.Dialog.Cancel

        QQC2.TextField {
            id: newName
            onAccepted: renamePrompt.accept()
        }

        onOpened: {
            const base = renamePrompt.path.includes("/")
                ? renamePrompt.path.substring(renamePrompt.path.lastIndexOf("/") + 1)
                : renamePrompt.path;
            newName.text = base.endsWith(".md") ? base.slice(0, -3) : base;
            newName.forceActiveFocus();
            newName.selectAll();
        }
        onAccepted: Nota.renamePath(path, newName.text)
    }

    Kirigami.PromptDialog {
        id: deletePrompt
        property string path: ""

        title: i18nc("@title:dialog", "Move to trash?")
        subtitle: i18n("“%1” goes to the trash inside your vault. Nothing is removed from disk.", deletePrompt.path)
        standardButtons: Kirigami.Dialog.Cancel

        customFooterActions: [
            Kirigami.Action {
                text: i18nc("@action:button", "Move to trash")
                icon.name: "edit-delete"
                onTriggered: {
                    Nota.removePath(deletePrompt.path);
                    deletePrompt.close();
                }
            }
        ]
    }
}
