// SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
// SPDX-License-Identifier: MIT
//
// The sidebar's context handling. Rename and Move to trash are enabled from
// sidebar.contextPath, so anything that clears it silently disables both.

import QtQuick
import QtTest

import org.kde.nota

Item {
    id: root
    width: 400
    height: 600

    Sidebar {
        id: sidebar
        anchors.fill: parent
    }

    TestCase {
        name: "Sidebar"
        when: windowShown

        function test_001_rightClickOnARowKeepsItAsTheContext(): void {
            const view = findChild(sidebar, "treeView");
            verify(view !== null, "the tree view must be findable");
            tryVerify(() => view.count > 0, 3000, "the sidebar must show the vault");

            const row = view.itemAtIndex(0);
            verify(row !== null, "the first row must be instantiated");
            const path = row.path;

            mouseClick(row, row.width / 2, row.height / 2, Qt.RightButton);
            compare(sidebar.contextPath, path);
        }

        function test_002_rightClickOnEmptySpaceTargetsTheRoot(): void {
            const view = findChild(sidebar, "treeView");
            tryVerify(() => view.count > 0, 3000);

            sidebar.contextPath = "Workplans";
            // Below the last row there is only the vault itself.
            mouseClick(view, view.width / 2, view.height - 4, Qt.RightButton);
            compare(sidebar.contextPath, "");
        }
    }
}
