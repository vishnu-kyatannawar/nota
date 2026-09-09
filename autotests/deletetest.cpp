/*
 * SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
 * SPDX-License-Identifier: MIT
 *
 * Deleting a page or a folder, from the sidebar's point of view.
 */

#include "app.h"
#include "foldertreemodel.h"
#include "itemmodel.h"
#include "settings.h"
#include "vault.h"

#include <QDir>
#include <QSignalSpy>
#include <QTemporaryDir>
#include <QTest>

using namespace Qt::StringLiterals;

class DeleteTest : public QObject
{
    Q_OBJECT

private Q_SLOTS:
    void init();
    void cleanup();

    void aWorkplanPageCanBeDeleted();
    void theWorkplanFolderItselfCannotBe();
    void deletingAFolderTakesItOutOfTheTree();
    void deletingAnEmptyFolderWorks();
    void deletingTheOpenPageLeavesNothingOpen();
    void deletingAFolderClosesAPageInsideIt();
    void aDeletedPageDoesNotComeBackFromAPendingSave();
    void aWorkplanPageCannotBeRenamed();
    void anOrdinaryPageFiledUnderWorkplansIsNotDated();
    void aFolderDeletedOutsideTheApplicationClosesThePageInsideIt();
    void savingDoesNotResetTheSidebar();

private:
    std::unique_ptr<QTemporaryDir> m_dir;
    std::unique_ptr<Nota> m_app;
};

void DeleteTest::init()
{
    m_dir = std::make_unique<QTemporaryDir>();
    QVERIFY(m_dir->isValid());
    Nota::setStartupVault(m_dir->path());
    m_app = std::make_unique<Nota>();
    QVERIFY(m_app->errorMessage().isEmpty());
}

void DeleteTest::cleanup()
{
    m_app.reset();
    m_dir.reset();
}

void DeleteTest::aWorkplanPageCanBeDeleted()
{
    // openToday() ran in the constructor, so there is a dated page to delete.
    const QString today = m_app->currentPath();
    QVERIFY(today.startsWith(u"Workplans/"_s));
    QVERIFY2(m_app->removePath(today), qPrintable(m_app->errorMessage()));
    QVERIFY(!QFile::exists(m_dir->path() + u'/' + today));
}

void DeleteTest::theWorkplanFolderItselfCannotBe()
{
    QVERIFY(!m_app->removePath(u"Workplans"_s));
    QVERIFY(QFile::exists(m_dir->path() + u"/Workplans"_s));
}

void DeleteTest::deletingAFolderTakesItOutOfTheTree()
{
    QVERIFY(m_app->createFolder({}, u"Projects"_s));
    m_app->createNote(u"Projects"_s);

    QVERIFY2(m_app->removePath(u"Projects"_s), qPrintable(m_app->errorMessage()));
    QVERIFY(!QFile::exists(m_dir->path() + u"/Projects"_s));

    // The sidebar reads the tree model, so the row has to be gone from it too.
    const FolderTreeModel *tree = m_app->folderTree();
    for (int row = 0; row < tree->rowCount(); ++row) {
        QCOMPARE_NE(tree->index(row, 0).data(FolderTreeModel::PathRole).toString(), u"Projects"_s);
    }
}

void DeleteTest::deletingAnEmptyFolderWorks()
{
    QVERIFY(m_app->createFolder({}, u"Empty"_s));
    QVERIFY2(m_app->removePath(u"Empty"_s), qPrintable(m_app->errorMessage()));
    QVERIFY(!QFile::exists(m_dir->path() + u"/Empty"_s));
}

void DeleteTest::deletingTheOpenPageLeavesNothingOpen()
{
    const QString page = m_app->createNote({});
    QCOMPARE(m_app->currentPath(), page);
    QVERIFY2(m_app->removePath(page), qPrintable(m_app->errorMessage()));
    QCOMPARE_NE(m_app->currentPath(), page);
}

void DeleteTest::deletingAFolderClosesAPageInsideIt()
{
    QVERIFY(m_app->createFolder({}, u"Projects"_s));
    const QString page = m_app->createNote(u"Projects"_s);
    QCOMPARE(m_app->currentPath(), page);

    QVERIFY2(m_app->removePath(u"Projects"_s), qPrintable(m_app->errorMessage()));
    QCOMPARE_NE(m_app->currentPath(), page);
}

void DeleteTest::aDeletedPageDoesNotComeBackFromAPendingSave()
{
    const QString page = m_app->createNote({});
    const int row = m_app->items()->insertItem(-1, 0);
    m_app->items()->setText(row, u"typed just before the delete"_s);
    QVERIFY(m_app->isDirty());

    QVERIFY2(m_app->removePath(page), qPrintable(m_app->errorMessage()));
    QTest::qWait(900); // longer than either debounce
    QVERIFY2(!QFile::exists(m_dir->path() + u'/' + page), "the debounced save recreated the deleted page");
}

void DeleteTest::aWorkplanPageCannotBeRenamed()
{
    // The filename is the date every lookup and the rollover find it by.
    const QString today = m_app->currentPath();
    QVERIFY(!m_app->renamePath(today, u"Some other name"_s));
    QVERIFY(QFile::exists(m_dir->path() + u'/' + today));
    QVERIFY(m_app->isDatedPage(today));
}

void DeleteTest::anOrdinaryPageFiledUnderWorkplansIsNotDated()
{
    // Only "Workplans/<date>.md" is dated. Anything else someone filed there
    // is an ordinary page and renames like one.
    QVERIFY(!m_app->isDatedPage(u"Workplans/Notes.md"_s));
    QVERIFY(!m_app->isDatedPage(u"Workplans/archive/2026-01-01.md"_s));
    QVERIFY(m_app->isDatedPage(u"Workplans/2026-01-01.md"_s));
    QVERIFY(!m_app->isReserved(u"Workplans/2026-01-01.md"_s));
    QVERIFY(m_app->isReserved(u"Workplans"_s));
}

void DeleteTest::aFolderDeletedOutsideTheApplicationClosesThePageInsideIt()
{
    // A vault is a directory people also use Dolphin and git on, so a folder
    // can go without this application being the one that removed it.
    QVERIFY(m_app->createFolder({}, u"Projects"_s));
    const QString page = m_app->createNote(u"Projects"_s);
    QCOMPARE(m_app->currentPath(), page);

    QVERIFY(QDir(m_dir->path() + u"/Projects"_s).removeRecursively());
    QTRY_VERIFY_WITH_TIMEOUT(m_app->currentPath() != page, 10000);
}

void DeleteTest::savingDoesNotResetTheSidebar()
{
    QVERIFY(m_app->createFolder({}, u"Projects"_s));
    const QString page = m_app->createNote(u"Projects"_s);
    QVERIFY(!page.isEmpty());

    // A reset collapses every folder the user expanded, so it must happen only
    // when the tree really changed — not on the save that follows a keystroke.
    QSignalSpy reset(m_app->folderTree(), &QAbstractItemModel::modelAboutToBeReset);
    const int row = m_app->items()->insertItem(-1, 0);
    m_app->items()->setText(row, u"a typed line"_s);
    m_app->flush();
    m_app->folderTree()->refresh();
    QCOMPARE(reset.count(), 0);

    // Deleting really does change it, so that one still resets.
    QVERIFY2(m_app->removePath(u"Projects"_s), qPrintable(m_app->errorMessage()));
    QCOMPARE(reset.count(), 1);
}

QTEST_MAIN(DeleteTest)
#include "deletetest.moc"
