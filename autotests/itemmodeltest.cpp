/*
 * SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
 * SPDX-License-Identifier: MIT
 *
 * The editing model.
 *
 * Every structural operation runs under QAbstractItemModelTester, so a
 * mismatched beginInsertRows/endInsertRows fails here rather than silently
 * destroying the focused delegate at runtime — which is the failure mode this
 * whole design exists to avoid.
 */

#include "itemmodel.h"
#include "mdnote.h"

#include <QAbstractItemModelTester>
#include <QSignalSpy>
#include <QTest>

using namespace Qt::StringLiterals;

class ItemModelTest : public QObject
{
    Q_OBJECT

private Q_SLOTS:
    void init();
    void cleanup();

    void splittingMidTextLeavesTheIdOnTheLeftHalf();
    void splittingAtTheEndAddsAnEmptyRowBelow();
    void splittingAtTheStartPushesAnEmptyRowAboveAndKeepsTheId();
    void mergingFoldsTextIntoTheRowAboveAndReportsTheCaret();
    void mergingRefusesOnTheFirstRowAndUnderAHeading();
    void indentCarriesTheWholeSubtree();
    void indentRefusesMoreThanOneLevelDeeperThanTheRowAbove();
    void indentStopsAtTheMaximumDepth();
    void outdentCarriesTheSubtreeAndRefusesAtTopLevel();
    void removingTakesTheDescendantsAndReturnsTheFocusRow();
    void insertingAfterARowReturnsTheNewRow();
    void togglingDoneFlipsOnlyActionItems();
    void headingsConvertBothWays();
    void editingTextIsVerbatim();
    void everyStructuralChangeAnnouncesItself();

private:
    void load(const QString &markdown);
    QString textAt(int row) const;
    int depthAt(int row) const;

    std::unique_ptr<ItemModel> m_model;
    std::unique_ptr<QAbstractItemModelTester> m_tester;
};

void ItemModelTest::init()
{
    m_model = std::make_unique<ItemModel>();
    m_tester = std::make_unique<QAbstractItemModelTester>(m_model.get(),
                                                          QAbstractItemModelTester::FailureReportingMode::QtTest);
}

void ItemModelTest::cleanup()
{
    m_tester.reset();
    m_model.reset();
}

void ItemModelTest::load(const QString &markdown)
{
    m_model->setItems(MdNote::parse(markdown).items);
}

QString ItemModelTest::textAt(int row) const
{
    return m_model->data(m_model->index(row, 0), ItemModel::TextRole).toString();
}

int ItemModelTest::depthAt(int row) const
{
    return m_model->data(m_model->index(row, 0), ItemModel::DepthRole).toInt();
}

void ItemModelTest::splittingMidTextLeavesTheIdOnTheLeftHalf()
{
    load(u"- [ ] Fix the auth bug <!--n id:A t:09:00-->\n"_s);

    // "Fix the" | " auth bug"
    const int focus = m_model->splitRow(0, 7);

    QCOMPARE(focus, 1);
    QCOMPARE(m_model->rowCount(), 2);
    QCOMPARE(textAt(0), u"Fix the"_s);
    QCOMPARE(textAt(1), u" auth bug"_s);
    // The original item keeps its identity and creation time with the left half.
    QCOMPARE(m_model->data(m_model->index(0, 0), ItemModel::IdRole).toString(), u"A"_s);
    QCOMPARE(m_model->data(m_model->index(0, 0), ItemModel::CreatedAtRole).toString(), u"09:00"_s);
    // The new half is a new item, so it gets its id when the note is saved.
    QVERIFY(m_model->data(m_model->index(1, 0), ItemModel::IdRole).toString().isEmpty());
}

void ItemModelTest::splittingAtTheEndAddsAnEmptyRowBelow()
{
    load(u"- [ ] Done thinking <!--n id:A-->\n"_s);

    const int focus = m_model->splitRow(0, 14);

    QCOMPARE(focus, 1);
    QCOMPARE(m_model->rowCount(), 2);
    QCOMPARE(textAt(1), QString());
    QCOMPARE(depthAt(1), 0);
}

void ItemModelTest::splittingAtTheStartPushesAnEmptyRowAboveAndKeepsTheId()
{
    load(u"- [ ] Keep me <!--n id:A t:09:00-->\n"_s);

    const int focus = m_model->splitRow(0, 0);

    QCOMPARE(m_model->rowCount(), 2);
    QCOMPARE(textAt(0), QString());
    QCOMPARE(textAt(1), u"Keep me"_s);
    // The id follows the text, not the position: focus stays on the real item.
    QCOMPARE(m_model->data(m_model->index(1, 0), ItemModel::IdRole).toString(), u"A"_s);
    QCOMPARE(focus, 1);
}

void ItemModelTest::mergingFoldsTextIntoTheRowAboveAndReportsTheCaret()
{
    load(u"- [ ] First <!--n id:A-->\n- [ ] second <!--n id:B-->\n"_s);

    const int caret = m_model->mergeWithPrevious(1);

    QCOMPARE(caret, 5); // the length of "First", where the caret belongs
    QCOMPARE(m_model->rowCount(), 1);
    QCOMPARE(textAt(0), u"Firstsecond"_s);
    QCOMPARE(m_model->data(m_model->index(0, 0), ItemModel::IdRole).toString(), u"A"_s);
}

void ItemModelTest::mergingRefusesOnTheFirstRowAndUnderAHeading()
{
    load(u"- [ ] Only <!--n id:A-->\n"_s);
    QCOMPARE(m_model->mergeWithPrevious(0), -1);
    QCOMPARE(m_model->rowCount(), 1);

    load(u"## Must\n\n- [ ] Under it <!--n id:A-->\n"_s);
    // Folding an item into a heading would silently turn work into a title.
    QCOMPARE(m_model->mergeWithPrevious(1), -1);
    QCOMPARE(m_model->rowCount(), 2);
}

void ItemModelTest::indentCarriesTheWholeSubtree()
{
    load(uR"(- [ ] Parent <!--n id:A-->
- [ ] Moves <!--n id:B-->
  - [ ] Child <!--n id:C-->
    - [ ] Grandchild <!--n id:D-->
- [ ] Untouched <!--n id:E-->
)"_s);

    QVERIFY(m_model->indentRow(1));

    QCOMPARE(depthAt(1), 1);
    QCOMPARE(depthAt(2), 2);
    QCOMPARE(depthAt(3), 3);
    QCOMPARE(depthAt(4), 0);
}

void ItemModelTest::indentRefusesMoreThanOneLevelDeeperThanTheRowAbove()
{
    load(u"- [ ] Parent <!--n id:A-->\n  - [ ] Child <!--n id:B-->\n"_s);

    // The child is already one deeper than its parent; two would be a gap.
    QVERIFY(!m_model->indentRow(1));
    QCOMPARE(depthAt(1), 1);
    // The very first row has nothing to nest under.
    QVERIFY(!m_model->indentRow(0));
    QCOMPARE(depthAt(0), 0);
}

void ItemModelTest::indentStopsAtTheMaximumDepth()
{
    QString markdown;
    for (int depth = 0; depth <= ItemModel::MaxDepth; ++depth) {
        markdown += QString(depth * MdNote::IndentUnit, u' ') + "- [ ] Level "_L1 + QString::number(depth) + u'\n';
    }
    load(markdown);

    const int last = m_model->rowCount() - 1;
    QCOMPARE(depthAt(last), ItemModel::MaxDepth);
    QVERIFY(!m_model->indentRow(last));
    QCOMPARE(depthAt(last), ItemModel::MaxDepth);
}

void ItemModelTest::outdentCarriesTheSubtreeAndRefusesAtTopLevel()
{
    load(uR"(- [ ] Parent <!--n id:A-->
  - [ ] Moves <!--n id:B-->
    - [ ] Child <!--n id:C-->
)"_s);

    QVERIFY(m_model->outdentRow(1));
    QCOMPARE(depthAt(1), 0);
    QCOMPARE(depthAt(2), 1);

    QVERIFY(!m_model->outdentRow(0));
    QCOMPARE(depthAt(0), 0);
}

void ItemModelTest::removingTakesTheDescendantsAndReturnsTheFocusRow()
{
    load(uR"(- [ ] Keep <!--n id:A-->
- [ ] Goes <!--n id:B-->
  - [ ] With it <!--n id:C-->
- [ ] Also keep <!--n id:D-->
)"_s);

    const int focus = m_model->removeRow(1);

    QCOMPARE(m_model->rowCount(), 2);
    QCOMPARE(textAt(0), u"Keep"_s);
    QCOMPARE(textAt(1), u"Also keep"_s);
    // Focus lands on the row above, which is where the caret was heading.
    QCOMPARE(focus, 0);
}

void ItemModelTest::insertingAfterARowReturnsTheNewRow()
{
    load(u"- [ ] One <!--n id:A-->\n- [ ] Two <!--n id:B-->\n"_s);

    const int focus = m_model->insertItem(0, 1);

    QCOMPARE(focus, 1);
    QCOMPARE(m_model->rowCount(), 3);
    QCOMPARE(textAt(1), QString());
    QCOMPARE(depthAt(1), 1);
    QCOMPARE(textAt(2), u"Two"_s);
}

void ItemModelTest::togglingDoneFlipsOnlyActionItems()
{
    load(u"## Must\n\n- [ ] Ship it <!--n id:A-->\n"_s);

    m_model->toggleDone(1);
    QVERIFY(m_model->data(m_model->index(1, 0), ItemModel::DoneRole).toBool());
    m_model->toggleDone(1);
    QVERIFY(!m_model->data(m_model->index(1, 0), ItemModel::DoneRole).toBool());

    // A heading has no checkbox, so this must be a no-op rather than a crash.
    m_model->toggleDone(0);
    QVERIFY(!m_model->data(m_model->index(0, 0), ItemModel::DoneRole).toBool());
}

void ItemModelTest::headingsConvertBothWays()
{
    load(u"- [ ] Must <!--n id:A t:09:00-->\n"_s);

    m_model->makeHeading(0);
    QVERIFY(m_model->data(m_model->index(0, 0), ItemModel::IsHeadingRole).toBool());
    QCOMPARE(m_model->data(m_model->index(0, 0), ItemModel::HeadingLevelRole).toInt(), 2);

    m_model->makeItem(0);
    QVERIFY(!m_model->data(m_model->index(0, 0), ItemModel::IsHeadingRole).toBool());
    QCOMPARE(textAt(0), u"Must"_s);
}

void ItemModelTest::editingTextIsVerbatim()
{
    load(u"- [ ] Old <!--n id:A-->\n"_s);

    // Labels and the [hh:mm] token live in the text; nothing may reorder them.
    m_model->setText(0, u"New #ops [01:20] trailing  "_s);
    QCOMPARE(textAt(0), u"New #ops [01:20] trailing  "_s);
    QCOMPARE(m_model->data(m_model->index(0, 0), ItemModel::LabelsRole).toStringList(), QStringList({u"ops"_s}));
    QCOMPARE(m_model->data(m_model->index(0, 0), ItemModel::MinutesRole).toInt(), 80);
}

void ItemModelTest::everyStructuralChangeAnnouncesItself()
{
    load(u"- [ ] One <!--n id:A-->\n- [ ] Two <!--n id:B-->\n"_s);
    QSignalSpy spy(m_model.get(), &ItemModel::changed);

    m_model->setText(0, u"Edited"_s);
    m_model->toggleDone(1);
    m_model->splitRow(0, 3);
    m_model->indentRow(1);
    m_model->removeRow(1);

    // The save pipeline is armed by this signal; a silent mutation is a lost edit.
    QCOMPARE(spy.count(), 5);
}

QTEST_GUILESS_MAIN(ItemModelTest)

#include "itemmodeltest.moc"
