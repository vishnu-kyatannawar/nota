/*
 * SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
 * SPDX-License-Identifier: MIT
 *
 * The pages column: what it shows, and in what order.
 */

#include "app.h"
#include "itemmodel.h"
#include "pagelistmodel.h"
#include "settings.h"

#include <QAbstractItemModelTester>
#include <QDir>
#include <QFile>
#include <QSignalSpy>
#include <QTemporaryDir>
#include <QTest>

using namespace Qt::StringLiterals;

class PageListTest : public QObject
{
    Q_OBJECT

private Q_SLOTS:
    void init();
    void cleanup();

    void workplansRunNewestFirst();
    void everyOtherFolderRunsByName();
    void theListFollowsTheSelectedFolder();
    void aPageCreatedInTheFolderArrivesAsOneRow();
    void savingDoesNotDisturbTheList();
    void deletingAPageTakesItsRowAway();

private:
    QStringList paths() const;
    void seedWorkplans(const QStringList &dates) const;

    std::unique_ptr<QTemporaryDir> m_dir;
    std::unique_ptr<Nota> m_app;
    std::unique_ptr<QAbstractItemModelTester> m_tester;
};

void PageListTest::seedWorkplans(const QStringList &dates) const
{
    const QString folder = m_dir->path() + "/Workplans"_L1;
    QDir().mkpath(folder);
    for (const QString &date : dates) {
        QFile f(folder + u'/' + date + ".md"_L1);
        if (f.open(QIODevice::WriteOnly)) {
            f.write(QStringLiteral("---\ntype: workplan\ndate: %1\n---\n\n").arg(date).toUtf8());
        }
    }
}

void PageListTest::init()
{
    m_dir = std::make_unique<QTemporaryDir>();
    QVERIFY(m_dir->isValid());
    // Seeded before the application opens, so today's rollover does not have
    // to be the only workplan there is.
    seedWorkplans({u"2026-08-03"_s, u"2026-08-31"_s, u"2026-09-07"_s});

    Nota::setStartupVault(m_dir->path());
    m_app = std::make_unique<Nota>();
    QVERIFY(m_app->errorMessage().isEmpty());
    m_tester = std::make_unique<QAbstractItemModelTester>(m_app->pages());
}

void PageListTest::cleanup()
{
    m_tester.reset();
    m_app.reset();
    m_dir.reset();
}

QStringList PageListTest::paths() const
{
    const PageListModel *pages = m_app->pages();
    QStringList out;
    for (int row = 0; row < pages->rowCount(); ++row) {
        out.append(pages->index(row, 0).data(PageListModel::PathRole).toString());
    }
    return out;
}

void PageListTest::workplansRunNewestFirst()
{
    // Launch opened today's workplan, so the list is already the workplans.
    QCOMPARE(m_app->pages()->folder(), u"Workplans"_s);

    const QStringList shown = paths();
    QVERIFY(shown.size() >= 3);

    // A workplan is named for its date, and the day anyone wants is the most
    // recent one — reading the oldest first is backwards for this folder only.
    QStringList descending = shown;
    std::sort(descending.begin(), descending.end(), std::greater<QString>());
    QCOMPARE(shown, descending);
    QCOMPARE(shown.constFirst(), u"Workplans/"_s + QDate::currentDate().toString(u"yyyy-MM-dd"_s) + u".md"_s);
}

void PageListTest::everyOtherFolderRunsByName()
{
    const QString folder = m_app->createFolder({}, u"Projects"_s);
    QVERIFY(!folder.isEmpty());
    QVERIFY(m_app->renamePath(m_app->createNote(folder), u"Zebra"_s));
    QVERIFY(m_app->renamePath(m_app->createNote(folder), u"Apple"_s));
    QVERIFY(m_app->renamePath(m_app->createNote(folder), u"Mango"_s));

    m_app->openFolder(folder);
    QCOMPARE(paths(), QStringList({u"Projects/Apple.md"_s, u"Projects/Mango.md"_s, u"Projects/Zebra.md"_s}));
}

void PageListTest::theListFollowsTheSelectedFolder()
{
    const QString folder = m_app->createFolder({}, u"Projects"_s);
    const QString page = m_app->createNote(folder);
    QVERIFY(!page.isEmpty());

    m_app->openFolder(u"Workplans"_s);
    QVERIFY(!paths().contains(page));

    m_app->openFolder(folder);
    QCOMPARE(paths(), QStringList({page}));
}

void PageListTest::aPageCreatedInTheFolderArrivesAsOneRow()
{
    const QString folder = m_app->createFolder({}, u"Projects"_s);
    m_app->openFolder(folder);

    PageListModel *pages = m_app->pages();
    QSignalSpy reset(pages, &QAbstractItemModel::modelAboutToBeReset);
    QSignalSpy inserted(pages, &QAbstractItemModel::rowsInserted);

    const QString page = m_app->createNote(folder);
    QVERIFY(!page.isEmpty());

    QCOMPARE(reset.count(), 0);
    QCOMPARE(inserted.count(), 1);
    QCOMPARE(pages->rowForPath(page), 0);
}

void PageListTest::savingDoesNotDisturbTheList()
{
    const QString folder = m_app->createFolder({}, u"Projects"_s);
    const QString page = m_app->createNote(folder);
    QVERIFY(!page.isEmpty());

    PageListModel *pages = m_app->pages();
    QSignalSpy reset(pages, &QAbstractItemModel::modelAboutToBeReset);
    QSignalSpy inserted(pages, &QAbstractItemModel::rowsInserted);
    QSignalSpy removed(pages, &QAbstractItemModel::rowsRemoved);

    const int row = m_app->items()->insertItem(-1, 0);
    m_app->items()->setText(row, u"a typed line"_s);
    m_app->flush();
    pages->refresh();

    // A save that emitted rows here would move the selection under the cursor.
    QCOMPARE(reset.count(), 0);
    QCOMPARE(inserted.count(), 0);
    QCOMPARE(removed.count(), 0);
}

void PageListTest::deletingAPageTakesItsRowAway()
{
    const QString folder = m_app->createFolder({}, u"Projects"_s);
    const QString page = m_app->createNote(folder);
    m_app->openFolder(folder);
    QCOMPARE(m_app->pages()->rowCount(), 1);

    PageListModel *pages = m_app->pages();
    QSignalSpy reset(pages, &QAbstractItemModel::modelAboutToBeReset);
    QSignalSpy removed(pages, &QAbstractItemModel::rowsRemoved);

    QVERIFY2(m_app->removePath(page), qPrintable(m_app->errorMessage()));

    QCOMPARE(reset.count(), 0);
    QCOMPARE(removed.count(), 1);
    QCOMPARE(pages->rowCount(), 0);
}

QTEST_MAIN(PageListTest)

#include "pagelisttest.moc"
