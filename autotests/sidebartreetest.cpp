/*
 * SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
 * SPDX-License-Identifier: MIT
 *
 * How the sidebar tree reports change.
 *
 * The distinction these suites exist for is reset versus rows. A reset is
 * correct in the sense that the model ends up right, and useless in the sense
 * that the view cannot tell the node it had expanded is the same node
 * afterwards — so every folder closes and the scroll position goes. Creating
 * one page inside one folder must not cost the user the shape of their tree.
 */

#include "app.h"
#include "foldertreemodel.h"
#include "itemmodel.h"

#include <QAbstractItemModelTester>
#include <QSignalSpy>
#include <QTemporaryDir>
#include <QTest>

using namespace Qt::StringLiterals;

class SidebarTreeTest : public QObject
{
    Q_OBJECT

private Q_SLOTS:
    void init();
    void cleanup();

    void savingEmitsNothingAtAll();
    void creatingAPageInAFolderInsertsOneRowUnderThatFolder();
    void creatingAFolderInsertsOneRowRatherThanResetting();
    void deletingRemovesOneRowRatherThanResetting();
    void aRenameIsAnInsertAndARemoveNotAReset();
    void openingAnEmptyFolderIsNotAnError();
    void openingAFolderThatHasPagesIsNotAnError();
    void openingAPageClearsTheFolderSelection();

private:
    QModelIndex indexFor(const QString &path) const;

    std::unique_ptr<QTemporaryDir> m_dir;
    std::unique_ptr<Nota> m_app;
    std::unique_ptr<QAbstractItemModelTester> m_tester;
};

void SidebarTreeTest::init()
{
    m_dir = std::make_unique<QTemporaryDir>();
    QVERIFY(m_dir->isValid());
    Nota::setStartupVault(m_dir->path());
    m_app = std::make_unique<Nota>();
    QVERIFY(m_app->errorMessage().isEmpty());

    // Every insert and remove below is checked against Qt's own rules for a
    // well-behaved model, which is the part hand-written row maths gets wrong.
    m_tester = std::make_unique<QAbstractItemModelTester>(m_app->folderTree());
}

void SidebarTreeTest::cleanup()
{
    m_tester.reset();
    m_app.reset();
    m_dir.reset();
}

QModelIndex SidebarTreeTest::indexFor(const QString &path) const
{
    const FolderTreeModel *tree = m_app->folderTree();
    for (int row = 0; row < tree->rowCount(); ++row) {
        const QModelIndex index = tree->index(row, 0);
        if (index.data(FolderTreeModel::PathRole).toString() == path) {
            return index;
        }
    }
    return {};
}

void SidebarTreeTest::savingEmitsNothingAtAll()
{
    const QString page = m_app->createNote({});
    QVERIFY(!page.isEmpty());

    FolderTreeModel *tree = m_app->folderTree();
    QSignalSpy reset(tree, &QAbstractItemModel::modelAboutToBeReset);
    QSignalSpy inserted(tree, &QAbstractItemModel::rowsInserted);
    QSignalSpy removed(tree, &QAbstractItemModel::rowsRemoved);

    const int row = m_app->items()->insertItem(-1, 0);
    m_app->items()->setText(row, u"a typed line"_s);
    m_app->flush();
    tree->refresh();

    QCOMPARE(reset.count(), 0);
    QCOMPARE(inserted.count(), 0);
    QCOMPARE(removed.count(), 0);
}

void SidebarTreeTest::creatingAPageInAFolderInsertsOneRowUnderThatFolder()
{
    QVERIFY(!m_app->createFolder({}, u"Projects"_s).isEmpty());
    const QModelIndex folder = indexFor(u"Projects"_s);
    QVERIFY(folder.isValid());

    FolderTreeModel *tree = m_app->folderTree();
    QSignalSpy reset(tree, &QAbstractItemModel::modelAboutToBeReset);
    QSignalSpy inserted(tree, &QAbstractItemModel::rowsInserted);

    const QString page = m_app->createNote(u"Projects"_s);
    QVERIFY(!page.isEmpty());

    QCOMPARE(reset.count(), 0);
    QCOMPARE(inserted.count(), 1);
    // The parent the row arrived under is what tells the view which folder to
    // keep open, so it has to be the folder and not the root.
    QCOMPARE(inserted.constFirst().at(0).toModelIndex(), folder);

    QCOMPARE(tree->rowCount(folder), 1);
    QCOMPARE(tree->index(0, 0, folder).data(FolderTreeModel::PathRole).toString(), page);
}

void SidebarTreeTest::creatingAFolderInsertsOneRowRatherThanResetting()
{
    FolderTreeModel *tree = m_app->folderTree();
    QSignalSpy reset(tree, &QAbstractItemModel::modelAboutToBeReset);
    QSignalSpy inserted(tree, &QAbstractItemModel::rowsInserted);

    QVERIFY(!m_app->createFolder({}, u"Projects"_s).isEmpty());

    QCOMPARE(reset.count(), 0);
    QCOMPARE(inserted.count(), 1);
    QVERIFY(indexFor(u"Projects"_s).isValid());
}

void SidebarTreeTest::deletingRemovesOneRowRatherThanResetting()
{
    QVERIFY(!m_app->createFolder({}, u"Projects"_s).isEmpty());
    QVERIFY(indexFor(u"Projects"_s).isValid());

    FolderTreeModel *tree = m_app->folderTree();
    QSignalSpy reset(tree, &QAbstractItemModel::modelAboutToBeReset);
    QSignalSpy removed(tree, &QAbstractItemModel::rowsRemoved);

    QVERIFY2(m_app->removePath(u"Projects"_s), qPrintable(m_app->errorMessage()));

    QCOMPARE(reset.count(), 0);
    QCOMPARE(removed.count(), 1);
    QVERIFY(!indexFor(u"Projects"_s).isValid());
}

void SidebarTreeTest::aRenameIsAnInsertAndARemoveNotAReset()
{
    const QString page = m_app->createNote({});
    QVERIFY(!page.isEmpty());

    FolderTreeModel *tree = m_app->folderTree();
    QSignalSpy reset(tree, &QAbstractItemModel::modelAboutToBeReset);

    QVERIFY2(m_app->renamePath(page, u"Renamed"_s), qPrintable(m_app->errorMessage()));

    QCOMPARE(reset.count(), 0);
    QVERIFY(indexFor(u"Renamed.md"_s).isValid());
    QVERIFY(!indexFor(page).isValid());
}

void SidebarTreeTest::openingAnEmptyFolderIsNotAnError()
{
    // A folder is a legitimate thing to click in a sidebar that shows folders.
    // Reading it as a note reported "file to open is a directory", which is
    // true and no help: what the user wanted was to put something in it.
    const QString folder = m_app->createFolder({}, u"R&D"_s);
    QVERIFY(!folder.isEmpty());

    m_app->clearError();
    m_app->open(folder);

    QVERIFY2(m_app->errorMessage().isEmpty(), qPrintable(m_app->errorMessage()));
    QCOMPARE(m_app->currentFolder(), folder);
    QVERIFY(m_app->currentPath().isEmpty());
}

void SidebarTreeTest::openingAFolderThatHasPagesIsNotAnError()
{
    const QString folder = m_app->createFolder({}, u"Projects"_s);
    QVERIFY(!folder.isEmpty());
    QVERIFY(!m_app->createNote(folder).isEmpty());

    m_app->clearError();
    m_app->open(folder);

    QVERIFY2(m_app->errorMessage().isEmpty(), qPrintable(m_app->errorMessage()));
    QCOMPARE(m_app->currentFolder(), folder);
}

void SidebarTreeTest::openingAPageClearsTheFolderSelection()
{
    const QString folder = m_app->createFolder({}, u"Projects"_s);
    const QString page = m_app->createNote(folder);
    QVERIFY(!page.isEmpty());

    m_app->open(folder);
    QCOMPARE(m_app->currentFolder(), folder);

    m_app->open(page);
    QCOMPARE(m_app->currentPath(), page);
    // Both showing at once would be two answers to "what am I looking at".
    QVERIFY(m_app->currentFolder().isEmpty());
}

QTEST_MAIN(SidebarTreeTest)

#include "sidebartreetest.moc"
