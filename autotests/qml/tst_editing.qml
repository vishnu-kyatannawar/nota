// SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
// SPDX-License-Identifier: MIT

import QtQuick
import QtTest

import org.kde.nota

Item {
    id: root
    width: 800
    height: 600

    ItemList {
        id: list
        anchors.fill: parent
    }

    TestCase {
        name: "Editing"
        when: windowShown

        function initTestCase(): void {
            // Launch rolled yesterday's items into today.
            tryVerify(() => list.count >= 3, 3000);
        }

        function cleanupTestCase(): void {
            Nota.flush();
        }

        function test_001_rowsHaveHeight(): void {
            const row = list.itemAtIndex(0);
            verify(row !== null, "the first row must be instantiated");
            // A delegate with only implicitHeight set is zero pixels tall, so
            // the whole list renders as nothing. That shipped once.
            verify(row.height > 0, "a row must have a height, got " + row.height);
            verify(list.contentHeight > 0, "the list must have content height");
        }

        function test_002_clickingARowFocusesItsField(): void {
            list.focusRow(0, 0);
            tryVerify(() => list.itemAtIndex(0).activeFocus, 2000, "row 0 should hold focus");
        }

        function test_003_typingReachesTheModel(): void {
            list.focusRow(0, 0);
            tryVerify(() => list.itemAtIndex(0).activeFocus, 2000);

            keyClick(Qt.Key_X);
            keyClick(Qt.Key_Y);
            // The field pushes into the model on a 150 ms debounce.
            tryVerify(() => list.model.data(list.model.index(0, 0), ItemModel.TextRole).toString().startsWith("xy"), 2000,
                      "typed characters must reach the model");
        }

        function test_004_enterCreatesARowBelow(): void {
            const before = list.count;
            list.focusRow(0, 0);
            tryVerify(() => list.itemAtIndex(0).activeFocus, 2000);

            keyClick(Qt.Key_End);
            keyClick(Qt.Key_Return);

            tryVerify(() => list.count === before + 1, 2000, "Enter must add a row");
            tryVerify(() => list.itemAtIndex(1).activeFocus, 2000, "the new row must take the caret");
        }

        function test_005_tabIndentsAndShiftTabLifts(): void {
            list.focusRow(2, 0);
            tryVerify(() => list.itemAtIndex(2).activeFocus, 2000);

            const depthRole = ItemModel.DepthRole;
            const before = list.model.data(list.model.index(2, 0), depthRole);

            keyClick(Qt.Key_Tab);
            tryVerify(() => list.model.data(list.model.index(2, 0), depthRole) === before + 1, 2000,
                      "Tab must nest the row");

            keyClick(Qt.Key_Backtab);
            tryVerify(() => list.model.data(list.model.index(2, 0), depthRole) === before, 2000,
                      "Shift+Tab must lift it back");
        }

        function test_006_ctrlEnterTicks(): void {
            list.focusRow(0, 0);
            tryVerify(() => list.itemAtIndex(0).activeFocus, 2000);

            const doneRole = ItemModel.DoneRole;
            const before = list.model.data(list.model.index(0, 0), doneRole);
            keyClick(Qt.Key_Return, Qt.ControlModifier);
            tryVerify(() => list.model.data(list.model.index(0, 0), doneRole) !== before, 2000,
                      "Ctrl+Enter must toggle done");
        }
    }
}
