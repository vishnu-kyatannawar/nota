// SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
// SPDX-License-Identifier: MIT

import QtQuick
import QtQuick.Layouts
import QtQuick.Controls as QQC2

import org.kde.kirigami as Kirigami
import org.kde.nota

// One editable row: an action item, or a group heading between items.
FocusScope {
    id: row

    required property int index
    required property string itemId
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

    property ListView view

    implicitHeight: layout.implicitHeight + Kirigami.Units.smallSpacing * 2

    // The model is the only thing that decides what a row says; this pushes
    // the field's text back into it, never the other way round while typing.
    function flush(): void {
        if (field.text !== row.text) {
            row.view.model.setText(row.index, field.text);
        }
    }

    function claimFocus(pos: int): void {
        field.forceActiveFocus(Qt.OtherFocusReason);
        if (pos < 0) {
            // A vertical move: keep the caret in the column it came from.
            const y = row.view.desiredColumnX >= 0 ? field.positionToRectangle(0).y : 0;
            field.cursorPosition = row.view.desiredColumnX >= 0
                ? field.positionAt(row.view.desiredColumnX, y)
                : field.length;
        } else {
            field.cursorPosition = Math.max(0, Math.min(pos, field.length));
        }
    }

    Component.onCompleted: {
        field.text = row.text;
        if (row.index === row.view.pendingFocusRow) {
            Qt.callLater(row.view.applyPendingFocus);
        }
    }

    Rectangle {
        anchors.fill: parent
        color: Kirigami.Theme.highlightColor
        opacity: field.activeFocus ? 0.08 : 0
        radius: Kirigami.Units.cornerRadius
    }

    RowLayout {
        id: layout
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        anchors.leftMargin: Kirigami.Units.largeSpacing + Kirigami.Units.gridUnit * row.depth
        anchors.rightMargin: Kirigami.Units.largeSpacing
        spacing: Kirigami.Units.smallSpacing

        QQC2.CheckBox {
            id: checkBox
            visible: !row.isHeading
            checked: row.done
            Layout.alignment: Qt.AlignTop
            onToggled: row.view.model.toggleDone(row.index)
        }

        ColumnLayout {
            Layout.fillWidth: true
            spacing: 2

            QQC2.TextArea {
                id: field

                // Width comes from the view, never from the delegate: taking it
                // from the parent closes a loop through implicitHeight and the
                // row collapses or grows without bound.
                Layout.preferredWidth: row.view.width
                    - Kirigami.Units.largeSpacing * 2
                    - Kirigami.Units.gridUnit * row.depth
                    - (row.isHeading ? 0 : checkBox.width + Kirigami.Units.smallSpacing)
                    - metaRow.implicitWidth - Kirigami.Units.smallSpacing
                Layout.fillWidth: true

                // Never MarkdownText: Qt's markdown writer normalises what it
                // reads, which would break the byte-for-byte round trip.
                textFormat: TextEdit.PlainText
                wrapMode: TextEdit.Wrap
                background: null
                padding: 0
                activeFocusOnTab: false
                placeholderText: row.index === 0 && row.text.length === 0 ? i18n("Type an item and press Enter") : ""

                font: row.isHeading ? Kirigami.Theme.defaultFont : Kirigami.Theme.defaultFont
                font.bold: row.isHeading
                font.pointSize: row.isHeading
                    ? Kirigami.Theme.defaultFont.pointSize * 1.15
                    : Kirigami.Theme.defaultFont.pointSize
                font.strikeout: row.done
                opacity: row.done ? 0.55 : 1

                onTextChanged: pushTimer.restart()
                onActiveFocusChanged: if (!activeFocus) {
                    row.flush();
                }

                Timer {
                    id: pushTimer
                    interval: 150
                    onTriggered: row.flush()
                }

                // The model changed underneath us — a reload, or a save that
                // minted ids. Only take it when we are not the one editing.
                Connections {
                    target: row.view.model
                    function onDataChanged(topLeft, bottomRight, roles): void {
                        if (field.activeFocus || row.index < topLeft.row || row.index > bottomRight.row) {
                            return;
                        }
                        if (field.text !== row.text) {
                            field.text = row.text;
                        }
                    }
                }

                Keys.onPressed: event => row.handleKey(event, field)
            }

            QQC2.Label {
                visible: row.hasBody
                text: row.body
                wrapMode: Text.Wrap
                font: Kirigami.Theme.fixedWidthFont
                opacity: 0.75
                Layout.fillWidth: true
            }
        }

        RowLayout {
            id: metaRow
            Layout.alignment: Qt.AlignTop
            spacing: Kirigami.Units.smallSpacing
            visible: !row.isHeading

            Kirigami.Icon {
                visible: row.recurring.length > 0
                source: "view-refresh-symbolic"
                implicitWidth: Kirigami.Units.iconSizes.small
                implicitHeight: Kirigami.Units.iconSizes.small
                opacity: 0.6
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

            QQC2.ToolButton {
                icon.name: "edit-delete-remove"
                display: QQC2.AbstractButton.IconOnly
                text: i18nc("@action:button", "Delete item")
                opacity: field.activeFocus ? 1 : 0
                onClicked: {
                    const focus = row.view.model.removeRow(row.index);
                    row.view.focusRow(focus, -1);
                }

                QQC2.ToolTip.text: text
                QQC2.ToolTip.visible: hovered
                QQC2.ToolTip.delay: Kirigami.Units.toolTipDelay
            }
        }
    }

    function handleKey(event, field): void {
        const view = row.view;
        const model = view.model;
        const mods = event.modifiers;

        // Ctrl+Enter — tick it done, stamping the time on save.
        if ((event.key === Qt.Key_Return || event.key === Qt.Key_Enter) && (mods & Qt.ControlModifier)) {
            model.toggleDone(row.index);
            event.accepted = true;
            return;
        }

        // Enter — split at the caret. At the start that means a row above; at
        // the end, an empty row below.
        if ((event.key === Qt.Key_Return || event.key === Qt.Key_Enter) && !(mods & Qt.ShiftModifier)) {
            row.flush();
            view.desiredColumnX = -1;
            view.focusRow(model.splitRow(row.index, field.cursorPosition), 0);
            event.accepted = true;
            return;
        }

        // Backspace at the very start folds this row into the one above.
        if (event.key === Qt.Key_Backspace && !(mods & Qt.ControlModifier)
            && field.cursorPosition === 0 && field.selectedText.length === 0) {
            row.flush();
            if (field.length === 0) {
                const focus = model.removeRow(row.index);
                view.focusRow(focus, -1);
                event.accepted = true;
                return;
            }
            const caret = model.mergeWithPrevious(row.index);
            if (caret >= 0) {
                view.focusRow(row.index - 1, caret);
            }
            event.accepted = true;
            return;
        }

        // Ctrl+Backspace / Ctrl+Delete — delete the item outright.
        if ((event.key === Qt.Key_Backspace || event.key === Qt.Key_Delete) && (mods & Qt.ControlModifier)) {
            row.flush();
            view.focusRow(model.removeRow(row.index), -1);
            event.accepted = true;
            return;
        }

        // Tab / Shift+Tab — indent and outdent, carrying the subtree.
        if (event.key === Qt.Key_Tab || event.key === Qt.Key_Backtab) {
            row.flush();
            const pos = field.cursorPosition;
            if (event.key === Qt.Key_Tab) {
                model.indentRow(row.index);
            } else {
                model.outdentRow(row.index);
            }
            view.focusRow(row.index, pos);
            event.accepted = true;
            return;
        }

        // Up / Down — leave the row only when the caret is already on its
        // first or last visual line, so a wrapped item still navigates inside.
        if (event.key === Qt.Key_Up || event.key === Qt.Key_Down) {
            const here = field.cursorRectangle;
            const first = field.positionToRectangle(0);
            const last = field.positionToRectangle(field.length);
            const atTop = Math.abs(here.y - first.y) < 1;
            const atBottom = Math.abs(here.y - last.y) < 1;
            if ((event.key === Qt.Key_Up && atTop) || (event.key === Qt.Key_Down && atBottom)) {
                row.flush();
                view.desiredColumnX = here.x;
                view.focusRow(row.index + (event.key === Qt.Key_Up ? -1 : 1), -1);
                event.accepted = true;
            }
            return;
        }

        // "## " on an empty row turns it into a group heading.
        if (event.key === Qt.Key_Space && field.text === "##" && !row.isHeading) {
            model.setText(row.index, "");
            model.makeHeading(row.index);
            field.text = "";
            event.accepted = true;
            return;
        }

        if (event.key === Qt.Key_Escape) {
            row.flush();
            field.focus = false;
            event.accepted = true;
        }
    }
}
