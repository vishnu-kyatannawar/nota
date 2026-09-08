// SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
// SPDX-License-Identifier: MIT

import QtQuick
import QtQuick.Controls as QQC2

import org.kde.kirigami as Kirigami
import org.kde.nota

// The outliner.
//
// Focus is driven from here rather than from the delegates: a delegate for a
// row that was just inserted does not exist in the same event-loop turn, so
// itemAtIndex() returns null and forcing focus directly does nothing. Instead
// the intent is recorded and the delegate claims it when it appears.
ListView {
    id: itemView

    model: Nota.items

    // Delegate recycling reuses the item that currently holds focus, and the
    // caret goes with it. A day has tens of items, not thousands, so the cost
    // of keeping them all alive is nil next to the bug it prevents.
    reuseItems: false
    cacheBuffer: Math.max(height * 3, 3000)
    // Up and Down have to reach the text field first, so it can decide whether
    // the caret is moving within a wrapped line or leaving the row.
    keyNavigationEnabled: false
    highlightMoveDuration: 0
    spacing: 0
    currentIndex: -1

    property int pendingFocusRow: -1
    property int pendingCursor: 0
    // Kept across a vertical move so the caret holds its visual column.
    property real desiredColumnX: -1

    function focusRow(row, pos): void {
        if (row < 0 || row >= count) {
            return;
        }
        currentIndex = row;
        pendingFocusRow = row;
        pendingCursor = pos;
        positionViewAtIndex(row, ListView.Contain);
        Qt.callLater(itemView.applyPendingFocus);
    }

    function applyPendingFocus(): void {
        if (pendingFocusRow < 0) {
            return;
        }
        const delegate = itemAtIndex(pendingFocusRow) as ItemRow;
        if (!delegate) {
            // Not instantiated yet; try again once the view has laid out.
            Qt.callLater(itemView.applyPendingFocus);
            return;
        }
        delegate.claimFocus(pendingCursor);
        pendingFocusRow = -1;
    }

    // Repeating items are seeded at the top of every workplan, so grouping on
    // the marker keeps them together without reordering anything.
    section.property: Nota.isWorkplan ? "isRepeating" : ""
    section.criteria: ViewSection.FullString
    section.delegate: Kirigami.ListSectionHeader {
        width: itemView.width
        text: section === "true"
            ? i18nc("@title:group items that come back every day", "Repeats daily")
            : i18nc("@title:group the rest of the day's work", "Today")
    }

    delegate: ItemRow {
        width: itemView.width
        view: itemView
    }

    Kirigami.PlaceholderMessage {
        anchors.centerIn: parent
        width: parent.width - Kirigami.Units.gridUnit * 4
        visible: itemView.count === 0 && Nota.currentPath.length > 0
        text: i18n("Nothing here yet")
        explanation: i18n("Press the button below, or start typing.")
    }
}
