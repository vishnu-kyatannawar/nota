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

    // The flattened row for a vault path, or -1 when it is not on screen. A
    // linear walk: the tree is a directory of markdown files, and this runs
    // once per page created.
    function rowForPath(path: string): int {
        for (let row = 0; row < flatTree.rowCount(); ++row) {
            if (flatTree.data(flatTree.index(row, 0), FolderTreeModel.PathRole) === path) {
                return row;
            }
        }
        return -1;
    }

    // Opens every folder above \a path so the thing just created is where the
    // user can see it. Top down, re-reading the row each time: a row does not
    // exist in the flattened model until its parent is expanded, so the rows
    // shift underneath as this goes.
    function reveal(path: string): void {
        if (path.length === 0) {
            return;
        }
        const parts = path.split("/");
        let prefix = "";
        for (let i = 0; i < parts.length - 1; ++i) {
            prefix = i === 0 ? parts[0] : prefix + "/" + parts[i];
            const row = rowForPath(prefix);
            if (row >= 0) {
                flatTree.expandChildren(row);
            }
        }
    }

    // Creates a page in \a folder and opens the tree down to it.
    function newPageIn(folder: string): void {
        const created = Nota.createNote(folder);
        if (created.length > 0) {
            reveal(created);
            contextPath = created;
            contextIsFolder = false;
        }
    }

    function newPage(): void {
        newPageIn(targetFolder());
    }

    /*! Asks for a new name for \a path. */
    function promptRename(path: string): void {
        renamePrompt.path = path;
        renamePrompt.open();
    }

    /*! Confirms, then moves \a path to the trash. */
    function promptTrash(path: string): void {
        deletePrompt.path = path;
        deletePrompt.open();
    }

    /*! Asks for a name, then creates the folder under \a parent. */
    function promptFolderIn(parent: string): void {
        folderPrompt.parentFolder = parent;
        folderPrompt.open();
    }

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
                onClicked: sidebar.newPage()
            }

            SidebarButton {
                icon.name: "folder-new"
                text: i18nc("@action:button", "New folder")
                onClicked: sidebar.promptFolderIn(sidebar.targetFolder())
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
                objectName: "treeView"
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
                            rowMenu.datedPath = Nota.isDatedPage(treeDelegate.path);
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

                // The empty space below the tree: a right-click there targets
                // the vault root. Delegates sit above this handler but do not
                // consume the tap, so without the indexAt() guard this fired
                // for a row as well and wiped the context the row just set —
                // which left Rename and Move to trash greyed out.
                TapHandler {
                    acceptedButtons: Qt.RightButton
                    onTapped: eventPoint => {
                        // eventPoint.position is already in the view's own
                        // coordinates; indexAt() wants the content's.
                        const local = eventPoint.position;
                        if (treeView.indexAt(treeView.contentX + local.x, treeView.contentY + local.y) >= 0) {
                            return;
                        }
                        sidebar.contextPath = "";
                        sidebar.contextIsFolder = true;
                        rowMenu.reservedPath = false;
                        rowMenu.datedPath = false;
                        rowMenu.popup();
                    }
                }
            }
        }

        Kirigami.Separator {
            Layout.fillWidth: true
        }

        // The vault and the running version. The version is here because the
        // first thing worth knowing about any report is which build produced
        // it, and an installed update does not take effect until the window is
        // reopened — so "what does the sidebar say" has to have an answer.
        RowLayout {
            Layout.fillWidth: true
            Layout.margins: Kirigami.Units.smallSpacing
            spacing: Kirigami.Units.smallSpacing

            QQC2.Label {
                Layout.fillWidth: true
                text: Nota.vaultPath
                elide: Text.ElideMiddle
                font: Kirigami.Theme.smallFont
                opacity: 0.7
            }

            Kirigami.Icon {
                source: "documentinfo-symbolic"
                implicitWidth: Kirigami.Units.iconSizes.small
                implicitHeight: Kirigami.Units.iconSizes.small
                opacity: 0.7
            }

            QQC2.Label {
                objectName: "versionLabel"
                text: Nota.version
                font: Kirigami.Theme.smallFont
                opacity: 0.7

                QQC2.ToolTip.visible: versionHover.hovered
                QQC2.ToolTip.text: i18nc("@info:tooltip", "Nota %1", Nota.version)
                HoverHandler {
                    id: versionHover
                }
            }
        }
    }

    QQC2.Menu {
        id: rowMenu

        // The workplan folder is reserved: renaming or deleting it would orphan
        // every dated note under it.
        property bool reservedPath: false

        // A dated note inside that folder. It can be thrown away like any
        // other page, but not renamed: the filename is the date.
        property bool datedPath: false

        QQC2.MenuItem {
            text: i18nc("@action:inmenu", "New page here")
            icon.name: "document-new"
            onTriggered: sidebar.newPage()
        }
        QQC2.MenuItem {
            text: i18nc("@action:inmenu", "New folder here")
            icon.name: "folder-new"
            onTriggered: sidebar.promptFolderIn(sidebar.targetFolder())
        }
        QQC2.MenuSeparator {}
        QQC2.MenuItem {
            text: i18nc("@action:inmenu", "Rename…")
            icon.name: "edit-rename"
            enabled: sidebar.contextPath.length > 0 && !rowMenu.reservedPath && !rowMenu.datedPath
            onTriggered: sidebar.promptRename(sidebar.contextPath)
        }
        QQC2.MenuItem {
            text: i18nc("@action:inmenu", "Move to trash")
            icon.name: "edit-delete"
            enabled: sidebar.contextPath.length > 0 && !rowMenu.reservedPath
            onTriggered: sidebar.promptTrash(sidebar.contextPath)
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
        onAccepted: {
            const created = Nota.createFolder(parentFolder, folderName.text);
            if (created.length > 0) {
                sidebar.reveal(created);
                sidebar.contextPath = created;
                sidebar.contextIsFolder = true;
            }
        }
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
                    if (Nota.removePath(deletePrompt.path) && sidebar.contextPath === deletePrompt.path) {
                        // Otherwise the context still names the folder that was
                        // just deleted, and "New page here" recreates it.
                        sidebar.contextPath = "";
                        sidebar.contextIsFolder = true;
                    }
                    deletePrompt.close();
                }
            }
        ]
    }
}
