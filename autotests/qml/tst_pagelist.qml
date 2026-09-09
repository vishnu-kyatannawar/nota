// SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
// SPDX-License-Identifier: MIT
//
// The pages column: the list itself, and the quick way to add to it.

import QtQuick
import QtTest

import org.kde.nota

Item {
    id: root
    width: 300
    height: 500

    property string askedForPageIn: ""

    PageList {
        id: pageList
        anchors.fill: parent
        onNewPageRequested: folder => root.askedForPageIn = folder
    }

    TestCase {
        name: "PageList"
        when: windowShown

        function test_001_theListShowsTheSelectedFolder(): void {
            const folder = Nota.createFolder("", "List Check");
            verify(folder.length > 0);
            const page = Nota.createNote(folder);
            verify(page.length > 0);

            Nota.openFolder(folder);
            compare(Nota.pages.folder, folder);

            const view = findChild(pageList, "pageList");
            verify(view !== null, "the page list must be findable");
            tryVerify(() => view.count === 1, 2000, "it must show the one page in the folder");

            const row = view.itemAtIndex(0);
            verify(row !== null, "the row must be instantiated");
            verify(row.height > 0, "a row must have a height, got " + row.height);
            compare(row.path, page);
        }

        function test_002_quickAddAsksForAPageInTheFolderOnScreen(): void {
            const folder = Nota.createFolder("", "Quick Add Check");
            verify(folder.length > 0);
            Nota.openFolder(folder);

            const button = findChild(pageList, "quickNewPage");
            verify(button !== null, "the quick add button must be there");
            verify(button.visible && button.width > 0, "and it must be on screen");

            root.askedForPageIn = "";
            mouseClick(button);
            compare(root.askedForPageIn, folder);
        }

        function test_003_clickingARowOpensThatPage(): void {
            const folder = Nota.createFolder("", "Open Check");
            const page = Nota.createNote(folder);
            verify(page.length > 0);
            Nota.openFolder(folder);

            const view = findChild(pageList, "pageList");
            tryVerify(() => view.count === 1, 2000);
            mouseClick(view.itemAtIndex(0));
            tryVerify(() => Nota.currentPath === page, 2000, "the row must open its page");
        }
    }
}
