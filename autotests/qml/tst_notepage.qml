// SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
// SPDX-License-Identifier: MIT
//
// What the page area shows when the thing selected is a folder. Reading a
// directory as a note used to put "file to open is a directory" across the top
// of the window, which is true and no help to anybody.

import QtQuick
import QtTest

import org.kde.nota

Item {
    id: root
    width: 700
    height: 500

    property string askedForPageIn: ""
    property string askedForFolderIn: ""

    NotePage {
        id: notePage
        anchors.fill: parent

        onNewPageRequested: folder => root.askedForPageIn = folder
        onNewFolderRequested: parentFolder => root.askedForFolderIn = parentFolder
    }

    TestCase {
        name: "NotePage"
        when: windowShown

        function test_001_anEmptyFolderOffersToFillIt(): void {
            const folder = Nota.createFolder("", "Placeholder Check");
            verify(folder.length > 0);

            Nota.open(folder);
            compare(Nota.currentFolder, folder);
            compare(Nota.errorMessage, "", "opening a folder must not raise an error");

            const placeholder = findChild(notePage, "folderPlaceholder");
            verify(placeholder !== null, "the folder placeholder must exist");
            tryVerify(() => placeholder.visible, 2000, "it must be shown for a folder");
            compare(placeholder.text, "Nothing here yet");
            compare(placeholder.explanation, folder);
        }

        function test_002_bothOffersAreWiredToTheFolderOnScreen(): void {
            const folder = Nota.createFolder("", "Offer Check");
            verify(folder.length > 0);
            Nota.open(folder);

            const newPage = findChild(notePage, "folderNewPage");
            const newFolder = findChild(notePage, "folderNewFolder");
            verify(newPage !== null && newFolder !== null, "both offers must be there");
            tryVerify(() => newPage.visible && newFolder.visible, 2000);

            mouseClick(newPage);
            compare(root.askedForPageIn, folder);

            mouseClick(newFolder);
            compare(root.askedForFolderIn, folder);
        }

        function test_003_openingAPageTakesThePlaceholderAway(): void {
            const folder = Nota.createFolder("", "Swap Check");
            Nota.open(folder);
            const placeholder = findChild(notePage, "folderPlaceholder");
            tryVerify(() => placeholder.visible, 2000);

            const page = Nota.createNote(folder);
            verify(page.length > 0);
            Nota.open(page);
            tryVerify(() => !placeholder.visible, 2000, "a page must replace the folder view");
        }
    }
}
