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
    onActiveChanged: {
        if (active) {
            Nota.openToday();
        } else {
            // Losing the window must not leave an edit sitting in a timer.
            Nota.flush();
        }
    }

    onClosing: Nota.flush()

    Shortcut {
        sequences: ["Ctrl+E"]
        onActivated: notePage.toggleRaw()
    }

    // The installer builds from source, so this is minutes rather than seconds
    // and the output is the only honest progress indicator there is.
    Kirigami.Dialog {
        id: updateDialog

        // Not "done" or "result": QQC2.Dialog already has a done() signal and
        // a FINAL result property, and shadowing either stops the whole window
        // being created — silently, unless you force Qt's logging to stderr.
        property bool complete: false
        property bool ok: false
        property string message: ""

        title: i18nc("@title:dialog", "Updating Nota")
        preferredWidth: Kirigami.Units.gridUnit * 34
        standardButtons: QQC2.Dialog.NoButton
        closePolicy: QQC2.Popup.NoAutoClose

        // Not "reset": that name collides with a superclass member, which QML
        // reports as an invalid override and then ignores the function.
        function prepare(): void {
            complete = false;
            ok = false;
            message = "";
            log.text = "";
        }

        customFooterActions: [
            Kirigami.Action {
                text: i18nc("@action:button", "Close")
                enabled: updateDialog.complete
                onTriggered: updateDialog.close()
            }
        ]

        ColumnLayout {
            spacing: Kirigami.Units.smallSpacing

            QQC2.Label {
                Layout.fillWidth: true
                wrapMode: Text.Wrap
                text: updateDialog.complete
                    ? (updateDialog.ok
                        ? i18n("Nota %1 is installed. Close and reopen the window to run it — this one keeps running the old build until you do.", Nota.updateVersion)
                        : updateDialog.message)
                    : i18n("Building Nota %1 from source. This takes a few minutes.", Nota.updateVersion)
            }

            QQC2.ScrollView {
                Layout.fillWidth: true
                Layout.preferredHeight: Kirigami.Units.gridUnit * 12

                QQC2.TextArea {
                    id: log
                    readOnly: true
                    wrapMode: Text.NoWrap
                    font.family: "monospace"
                    font.pointSize: Kirigami.Theme.smallFont.pointSize
                }
            }
        }

        Connections {
            target: Nota

            function onUpdateOutput(line: string): void {
                log.text += line + "\n";
                // Follow the tail: the interesting line is always the last one.
                log.cursorPosition = log.length;
            }

            function onUpdateFinished(ok: bool, message: string): void {
                updateDialog.complete = true;
                updateDialog.ok = ok;
                updateDialog.message = message;
            }
        }
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

            // What can be offered depends on who owns the binary: replacing a
            // packaged install would leave pacman or dnf describing files that
            // are no longer the ones on disk, so there it is a note, not a
            // button. Dismissal lasts the session; there is no nagging state
            // to keep, and the next launch checks again anyway.
            Kirigami.InlineMessage {
                id: updateNotice

                property bool dismissed: false

                Layout.fillWidth: true
                type: Kirigami.MessageType.Information
                visible: !dismissed && Nota.updateVersion.length > 0
                showCloseButton: true
                text: Nota.canSelfUpdate
                    ? i18n("Nota %1 is available. You are running %2.", Nota.updateVersion, Nota.version)
                    : i18n("Nota %1 is available. This copy came from your package manager, so update it there.",
                           Nota.updateVersion)
                onVisibleChanged: if (!visible) {
                    updateNotice.dismissed = true;
                }

                actions: [
                    Kirigami.Action {
                        text: i18nc("@action:button", "Update")
                        icon.name: "install-symbolic"
                        visible: Nota.canSelfUpdate
                        enabled: !Nota.updateRunning
                        onTriggered: {
                            updateDialog.prepare();
                            updateDialog.open();
                            Nota.startUpdate();
                        }
                    },
                    Kirigami.Action {
                        text: i18nc("@action:button", "What's changed")
                        icon.name: "documentinfo-symbolic"
                        visible: Nota.updateUrl.length > 0
                        onTriggered: Qt.openUrlExternally(Nota.updateUrl)
                    }
                ]
            }

            // A SplitView rather than fixed widths: how much room a folder
            // name or a page title needs depends on what someone called them,
            // which this cannot know. The two edges are draggable and the
            // widths are remembered per machine.
            QQC2.SplitView {
                id: columns
                objectName: "columns"

                Layout.fillWidth: true
                Layout.fillHeight: true
                orientation: Qt.Horizontal

                handle: Rectangle {
                    implicitWidth: Kirigami.Units.smallSpacing
                    // Kirigami's theme has no separator colour; the faint one
                    // between rows is what a divider is drawn from here.
                    color: QQC2.SplitHandle.pressed || QQC2.SplitHandle.hovered
                        ? Kirigami.Theme.highlightColor
                        : Kirigami.Theme.alternateBackgroundColor

                    // The line is a hair wide; the grab area cannot be, or the
                    // handle is a pixel hunt.
                    HoverHandler {
                        cursorShape: Qt.SplitHCursor
                        margin: Kirigami.Units.smallSpacing
                    }
                }

                Sidebar {
                    id: sidebar
                    objectName: "folderColumn"

                    QQC2.SplitView.minimumWidth: Kirigami.Units.gridUnit * 6
                    QQC2.SplitView.maximumWidth: Kirigami.Units.gridUnit * 30

                    // Assigned once rather than bound: this is also written
                    // back as the user drags, and a binding both ways is a loop.
                    Component.onCompleted: {
                        QQC2.SplitView.preferredWidth = Nota.folderColumnWidth > 0
                            ? Nota.folderColumnWidth
                            : Kirigami.Units.gridUnit * 13;
                    }
                }

                PageList {
                    id: pageList
                    objectName: "pageColumn"

                    QQC2.SplitView.minimumWidth: Kirigami.Units.gridUnit * 6
                    QQC2.SplitView.maximumWidth: Kirigami.Units.gridUnit * 34

                    onNewPageRequested: folder => sidebar.newPageIn(folder)
                    onRenameRequested: path => sidebar.promptRename(path)
                    onTrashRequested: path => sidebar.promptTrash(path)

                    Component.onCompleted: {
                        QQC2.SplitView.preferredWidth = Nota.pageColumnWidth > 0
                            ? Nota.pageColumnWidth
                            : Kirigami.Units.gridUnit * 14;
                    }
                }

                NotePage {
                    id: notePage

                    QQC2.SplitView.fillWidth: true
                    QQC2.SplitView.minimumWidth: Kirigami.Units.gridUnit * 16

                    // The sidebar creates and then reveals; the page only asks.
                    onNewPageRequested: folder => sidebar.newPageIn(folder)
                    onNewFolderRequested: parentFolder => sidebar.promptFolderIn(parentFolder)
                }

                // Dragging emits a width for every pixel crossed. Writing the
                // state file that often would be absurd, so it is written once
                // the handle has been still for a moment.
                Timer {
                    id: rememberWidths
                    interval: 400
                    onTriggered: {
                        Nota.folderColumnWidth = sidebar.width;
                        Nota.pageColumnWidth = pageList.width;
                    }
                }

                Connections {
                    target: sidebar
                    function onWidthChanged(): void {
                        rememberWidths.restart();
                    }
                }

                Connections {
                    target: pageList
                    function onWidthChanged(): void {
                        rememberWidths.restart();
                    }
                }
            }
        }
    }
}
