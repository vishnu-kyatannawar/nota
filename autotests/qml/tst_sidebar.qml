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

        function test_004_creatingRevealsTheNewFolderAndKeepsTheRestOpen(): void {
            const view = findChild(sidebar, "treeView");
            tryVerify(() => view.count > 0, 3000);

            // A folder nested inside one that is closed has no row at all
            // until its parent is expanded.
            const outer = Nota.createFolder("", "Outer");
            const inner = Nota.createFolder(outer, "Inner");
            verify(outer.length > 0 && inner.length > 0);

            sidebar.reveal(inner);
            verify(sidebar.rowForPath(inner) >= 0, "the new folder must be revealed");

            // Making something else must not close what is already open.
            const elsewhere = Nota.createFolder("", "Elsewhere");
            verify(elsewhere.length > 0);
            verify(sidebar.rowForPath(inner) >= 0,
                   "creating a folder elsewhere must not collapse the rest of the tree");
        }

        function test_005_pagesLiveInTheirOwnColumnNotTheTree(): void {
            const folder = Nota.createFolder("", "Column Check");
            verify(folder.length > 0);

            const page = Nota.createNote(folder);
            verify(page.length > 0);

            // The tree is folders. The page belongs to the list beside it.
            compare(sidebar.rowForPath(page), -1, "a page must not be a row in the folder tree");
            compare(Nota.pages.folder, folder);
            verify(Nota.pages.rowForPath(page) >= 0, "the page must be in the page list");
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
