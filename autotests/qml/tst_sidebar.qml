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

        function test_003_theRunningVersionIsOnScreen(): void {
            // The answer to "which build are you on" has to be visible without
            // a terminal, because an installed update only takes effect on the
            // next launch and a stale window looks exactly like an unfixed bug.
            const label = findChild(sidebar, "versionLabel");
            verify(label !== null, "the sidebar must show a version");
            verify(label.text.length > 0, "the version must not be blank");
            compare(label.text, Nota.version);
            // This project has shipped a zero-height row before, so "present
            // in the tree" is not the same claim as "on screen".
            verify(label.width > 0 && label.height > 0, "the version label must have a size");
        }

        function test_004_creatingInAFolderOpensItAndKeepsTheRestExpanded(): void {
            const view = findChild(sidebar, "treeView");
            tryVerify(() => view.count > 0, 3000);

            // Two folders, one expanded by hand, the other left closed.
            const open = Nota.createFolder("", "Open");
            const shut = Nota.createFolder("", "Shut");
            verify(open.length > 0 && shut.length > 0);

            const openRow = sidebar.rowForPath(open);
            verify(openRow >= 0, "the expanded folder must be on screen");
            Nota.createNote(open);
            sidebar.reveal(open + "/Untitled.md");
            const beforeCount = view.count;
            verify(sidebar.rowForPath(open + "/Untitled.md") >= 0, "the first page must be visible");

            // A page created in the closed folder: the folder has to open so
            // the new page is where the user can see it...
            const page = Nota.createNote(shut);
            sidebar.reveal(page);
            verify(sidebar.rowForPath(page) >= 0, "the new page must be revealed, got no row for " + page);

            // ...and the folder that was already open must still be open.
            verify(sidebar.rowForPath(open + "/Untitled.md") >= 0,
                   "creating a page elsewhere must not collapse the rest of the tree");
            verify(view.count >= beforeCount, "rows must not have disappeared");
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
