/*
 * SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
 * SPDX-License-Identifier: MIT
 */

#include "mdnote.h"

#include <QDir>
#include <QFile>
#include <QTest>

using namespace Qt::StringLiterals;
using namespace MdNote;

namespace
{
QString read(const QString &path)
{
    QFile f(path);
    if (!f.open(QIODevice::ReadOnly | QIODevice::Text)) {
        return {};
    }
    return QString::fromUtf8(f.readAll());
}
} // namespace

class MdNoteTest : public QObject
{
    Q_OBJECT

private Q_SLOTS:
    void goldenFilesRoundTripByteForByte_data();
    void goldenFilesRoundTripByteForByte();
    void serializeIsIdempotent_data();
    void serializeIsIdempotent();

    void parsesFrontmatter();
    void parsesItemsWithTheirMetadata();
    void parsesNestedItemsAndBodies();
    void fencedCodeIsNotParsedAsItems();
    void labelsAndTimeStayVisibleInText();
    void setMinutesReplacesInPlace();
    void durationsRoundTrip();
    void noteLabelsComeFromFrontmatterAndItems();
    void crlfInputIsNormalised();

    void addAndFindItem();
    void setDoneStampsAndClears();
    void removeItemAlsoRemovesItsChildren();
    void addItemMinutesAccumulatesAndFloorsAtZero();

    void replaceItemsKeepsFrontmatterAndBody();
    void replaceItemsStampsDoneTransitions();
    void replaceItemsPreservesAnExistingDoneStamp();
    void replaceItemsKeepsRolloverMetadataById();
    void replaceItemsPassesHeadingsThrough();

    void layoutParsesAndDefaults();
    void takeItemsMovesSubtreesAndRebasesDepth();
    void takeItemsChildAloneBecomesTopLevel();
    void takeItemsUnknownIdIsANoOp();
    void appendItemsGoesToTheEnd();

    void headingsBetweenItemsAreGroupHeadings();
    void headingFollowedByProseIsBody();
    void consecutiveHeadingsBeforeAnItem();
    void headingsCarryNoLabelsAndAreNeverTaken();

private:
    static QStringList goldenFiles();
};

QStringList MdNoteTest::goldenFiles()
{
    QDir dir(QStringLiteral(NOTA_TEST_DATA_DIR));
    return dir.entryList({QStringLiteral("*.md")}, QDir::Files, QDir::Name);
}

void MdNoteTest::goldenFilesRoundTripByteForByte_data()
{
    QTest::addColumn<QString>("file");
    for (const QString &name : goldenFiles()) {
        QTest::newRow(qPrintable(name)) << name;
    }
}

void MdNoteTest::goldenFilesRoundTripByteForByte()
{
    QFETCH(QString, file);
    const QString source = read(QStringLiteral(NOTA_TEST_DATA_DIR "/") + file);
    QVERIFY2(!source.isEmpty(), "the fixture should not be empty");
    QCOMPARE(serialize(parse(source)), source);
}

void MdNoteTest::serializeIsIdempotent_data()
{
    goldenFilesRoundTripByteForByte_data();
}

void MdNoteTest::serializeIsIdempotent()
{
    QFETCH(QString, file);
    const QString once = serialize(parse(read(QStringLiteral(NOTA_TEST_DATA_DIR "/") + file)));
    QCOMPARE(serialize(parse(once)), once);
}

void MdNoteTest::parsesFrontmatter()
{
    const Note n = parse(uR"(---
id: 01K6M2QW8ZP4
type: workplan
date: 2026-09-02
hours: "01:20"
daytype: work
labels: [work, ops]
---

- [ ] Something <!--n id:A t:09:00-->
)"_s);

    QVERIFY(n.hadFrontmatter);
    QCOMPARE(n.id, "01K6M2QW8ZP4"_L1);
    QCOMPARE(n.type, "workplan"_L1);
    QCOMPARE(n.date, "2026-09-02"_L1);
    QCOMPARE(n.hours, "01:20"_L1);
    QCOMPARE(n.dayType, "work"_L1);
    QCOMPARE(n.labels, QStringList({u"work"_s, u"ops"_s}));
}

void MdNoteTest::parsesItemsWithTheirMetadata()
{
    const Note n = parse(uR"(- [x] Fix auth #rv-api [01:20] <!--n id:01K6M2R4 t:09:34 done:11:02 from:2026-09-01 carried:3 rec:daily-->
)"_s);

    QCOMPARE(n.items.size(), 1);
    const Item &it = n.items.first();
    QVERIFY(it.done);
    QCOMPARE(it.text, u"Fix auth #rv-api [01:20]"_s);
    QCOMPARE(it.id, "01K6M2R4"_L1);
    // The clock values survive being split on their own colon.
    QCOMPARE(it.createdAt, "09:34"_L1);
    QCOMPARE(it.doneAt, "11:02"_L1);
    QCOMPARE(it.from, "2026-09-01"_L1);
    QCOMPARE(it.carried, 3);
    QCOMPARE(it.recurring, "daily"_L1);
}

void MdNoteTest::parsesNestedItemsAndBodies()
{
    const Note n = parse(uR"(- [ ] Parent <!--n id:A t:09:00-->
      Some body text.

      More of it.
  - [ ] Child <!--n id:B t:09:01-->
    - [ ] Grandchild <!--n id:C t:09:02-->
)"_s);

    QCOMPARE(n.items.size(), 3);
    QCOMPARE(n.items.at(0).depth, 0);
    QCOMPARE(n.items.at(1).depth, 1);
    QCOMPARE(n.items.at(2).depth, 2);
    QCOMPARE(n.items.at(0).body, QStringList({u"Some body text."_s, QString(), u"More of it."_s}));
    QVERIFY(n.items.at(1).body.isEmpty());
}

void MdNoteTest::fencedCodeIsNotParsedAsItems()
{
    const QString src = read(QStringLiteral(NOTA_TEST_DATA_DIR "/code-fence-with-checkboxes.md"));
    const Note n = parse(src);

    // One real item; the two checklist lines inside the fence are body.
    QCOMPARE(n.items.size(), 1);
    QVERIFY(n.items.first().body.join(u'\n').contains(u"- [ ] this is sample content"_s));
    QCOMPARE(serialize(n), src);
}

void MdNoteTest::labelsAndTimeStayVisibleInText()
{
    Item it;
    it.text = u"Ship it #rv-api #ops [01:30] but `#not-a-label` in code"_s;

    QCOMPARE(it.labels(), QStringList({u"rv-api"_s, u"ops"_s}));
    QCOMPARE(it.minutes(), 90);
    // The text is never rebuilt from its parts.
    QVERIFY(it.text.contains(u"[01:30]"_s));
}

void MdNoteTest::setMinutesReplacesInPlace()
{
    Item it;
    it.text = u"Fix auth [01:20] #rv-api"_s;
    it.setMinutes(150);
    QCOMPARE(it.text, u"Fix auth [02:30] #rv-api"_s);

    // Zero removes the token rather than writing a meaningless [00:00].
    it.setMinutes(0);
    QCOMPARE(it.text, u"Fix auth #rv-api"_s);

    // A first log is appended at the end.
    it.setMinutes(45);
    QCOMPARE(it.text, u"Fix auth #rv-api [00:45]"_s);
}

void MdNoteTest::durationsRoundTrip()
{
    int minutes = -1;
    QVERIFY(parseDuration(u"01:20"_s, &minutes));
    QCOMPARE(minutes, 80);
    QVERIFY(parseDuration(u"123:59"_s, &minutes));
    QCOMPARE(minutes, 123 * 60 + 59);

    QVERIFY(!parseDuration(u"1:20"_s, &minutes));
    QVERIFY(!parseDuration(u"01:60"_s, &minutes));
    QVERIFY(!parseDuration(u"nonsense"_s, &minutes));

    QCOMPARE(formatDuration(80), u"01:20"_s);
    QCOMPARE(formatDuration(0), u"00:00"_s);
    QCOMPARE(formatDuration(-5), u"00:00"_s);
}

void MdNoteTest::noteLabelsComeFromFrontmatterAndItems()
{
    const Note n = parse(uR"(---
labels: [reference]
---

- [ ] One #ops <!--n id:A-->
- [ ] Two #ops #billing <!--n id:B-->
)"_s);

    QCOMPARE(n.allLabels(), QStringList({u"billing"_s, u"ops"_s, u"reference"_s}));
}

void MdNoteTest::crlfInputIsNormalised()
{
    const Note n = parse(u"- [ ] One <!--n id:A-->\r\n- [ ] Two <!--n id:B-->\r\n"_s);
    QCOMPARE(n.items.size(), 2);
    QCOMPARE(serialize(n), u"- [ ] One <!--n id:A-->\n- [ ] Two <!--n id:B-->\n"_s);
}

void MdNoteTest::addAndFindItem()
{
    Note n;
    n.addItem(u"A"_s, u"  Padded  "_s, u"09:00"_s, 1);
    QCOMPARE(n.indexOfItem(u"A"_s), 0);
    QCOMPARE(n.indexOfItem(u"missing"_s), -1);
    QCOMPARE(n.items.first().text, u"Padded"_s);
    QCOMPARE(n.items.first().depth, 1);
}

void MdNoteTest::setDoneStampsAndClears()
{
    Note n;
    n.addItem(u"A"_s, u"One"_s, u"09:00"_s, 0);

    QVERIFY(n.setDone(u"A"_s, true, u"11:02"_s));
    QCOMPARE(n.items.first().doneAt, u"11:02"_s);

    QVERIFY(n.setDone(u"A"_s, false, u"12:00"_s));
    QVERIFY(n.items.first().doneAt.isEmpty());

    QVERIFY(!n.setDone(u"missing"_s, true, u"12:00"_s));
}

void MdNoteTest::removeItemAlsoRemovesItsChildren()
{
    Note n = parse(uR"(- [ ] Parent <!--n id:A-->
  - [ ] Child <!--n id:B-->
    - [ ] Grandchild <!--n id:C-->
- [ ] Sibling <!--n id:D-->
)"_s);

    QVERIFY(n.removeItem(u"A"_s));
    QCOMPARE(n.items.size(), 1);
    QCOMPARE(n.items.first().id, u"D"_s);
}

void MdNoteTest::addItemMinutesAccumulatesAndFloorsAtZero()
{
    Note n;
    n.addItem(u"A"_s, u"One"_s, u"09:00"_s, 0);

    QVERIFY(n.addItemMinutes(u"A"_s, 30));
    QCOMPARE(n.items.first().minutes(), 30);
    QVERIFY(n.addItemMinutes(u"A"_s, 45));
    QCOMPARE(n.items.first().minutes(), 75);
    QVERIFY(n.addItemMinutes(u"A"_s, -1000));
    QCOMPARE(n.items.first().minutes(), 0);
}

void MdNoteTest::replaceItemsKeepsFrontmatterAndBody()
{
    Note n = parse(uR"(---
type: workplan
date: 2026-09-02
hours: "01:20"
daytype: work
---

- [ ] Old <!--n id:A t:09:00-->

## Notes

Prose that must survive.
)"_s);

    Item replacement;
    replacement.id = u"B"_s;
    replacement.text = u"New"_s;
    n.replaceItemsAt({replacement}, u"10:00"_s);

    QCOMPARE(n.hours, u"01:20"_s);
    QCOMPARE(n.date, u"2026-09-02"_s);
    QVERIFY(n.body.contains(u"Prose that must survive."_s));
    QCOMPARE(n.items.size(), 1);
    QCOMPARE(n.items.first().id, u"B"_s);
}

void MdNoteTest::replaceItemsStampsDoneTransitions()
{
    Note n = parse(u"- [ ] One <!--n id:A t:09:00-->\n"_s);

    Item ticked = n.items.first();
    ticked.done = true;
    n.replaceItemsAt({ticked}, u"11:02"_s);
    QCOMPARE(n.items.first().doneAt, u"11:02"_s);

    // Reopening clears the stamp, so an item never claims to be finished.
    Item reopened = n.items.first();
    reopened.done = false;
    n.replaceItemsAt({reopened}, u"12:00"_s);
    QVERIFY(n.items.first().doneAt.isEmpty());
}

void MdNoteTest::replaceItemsPreservesAnExistingDoneStamp()
{
    Note n = parse(u"- [x] One <!--n id:A t:09:00 done:11:02-->\n"_s);

    Item unchanged = n.items.first();
    unchanged.text = u"One, reworded"_s;
    n.replaceItemsAt({unchanged}, u"15:00"_s);

    QCOMPARE(n.items.first().doneAt, u"11:02"_s);
}

void MdNoteTest::replaceItemsKeepsRolloverMetadataById()
{
    Note n = parse(u"- [ ] One <!--n id:A t:09:00 from:2026-09-01 carried:4 rec:standup-->\n"_s);

    // The editor only ever sends back id, text, done, depth and body.
    Item fromEditor;
    fromEditor.id = u"A"_s;
    fromEditor.text = u"One, edited"_s;
    n.replaceItemsAt({fromEditor}, u"10:00"_s);

    const Item &it = n.items.first();
    QCOMPARE(it.text, u"One, edited"_s);
    QCOMPARE(it.createdAt, u"09:00"_s);
    QCOMPARE(it.from, u"2026-09-01"_s);
    QCOMPARE(it.carried, 4);
    QCOMPARE(it.recurring, u"standup"_s);
}

void MdNoteTest::replaceItemsPassesHeadingsThrough()
{
    Note n;
    Item heading;
    heading.kind = KindHeading;
    heading.level = 0; // out of range, so it is written as 2
    heading.text = u"  Must  "_s;

    Item item;
    item.id = u"A"_s;
    item.text = u"Ship it"_s;

    n.replaceItemsAt({heading, item}, u"09:00"_s);

    QCOMPARE(n.items.size(), 2);
    QVERIFY(n.items.first().isHeading());
    QCOMPARE(n.items.first().level, 2);
    QCOMPARE(n.items.first().text, u"Must"_s);
}

void MdNoteTest::layoutParsesAndDefaults()
{
    QCOMPARE(parse(u"---\nlayout: items\n---\n"_s).effectiveLayout(), LayoutItems);
    QCOMPARE(parse(u"---\nlayout: notes\n---\n"_s).effectiveLayout(), LayoutNotes);
    QCOMPARE(parse(u"---\nlayout: nonsense\n---\n"_s).effectiveLayout(), LayoutBoth);
    QCOMPARE(parse(u"# Just prose\n"_s).effectiveLayout(), LayoutBoth);
    // A workplan is always both, whatever the frontmatter says.
    QCOMPARE(parse(u"---\ntype: workplan\nlayout: items\n---\n"_s).effectiveLayout(), LayoutBoth);
}

void MdNoteTest::takeItemsMovesSubtreesAndRebasesDepth()
{
    Note n = parse(uR"(- [ ] Keep <!--n id:A-->
- [ ] Move <!--n id:B t:09:40 from:2026-09-01 carried:1-->
  - [ ] Child <!--n id:C-->
- [ ] Keep too <!--n id:D-->
)"_s);

    const QList<Item> taken = n.takeItems({u"B"_s});

    QCOMPARE(taken.size(), 2);
    QCOMPARE(taken.at(0).id, u"B"_s);
    QCOMPARE(taken.at(0).depth, 0);
    QCOMPARE(taken.at(1).depth, 1);
    // Metadata travels with the item, so it still says when it was born.
    QCOMPARE(taken.at(0).from, u"2026-09-01"_s);
    QCOMPARE(taken.at(0).carried, 1);

    QCOMPARE(n.items.size(), 2);
    QCOMPARE(n.items.at(0).id, u"A"_s);
    QCOMPARE(n.items.at(1).id, u"D"_s);
}

void MdNoteTest::takeItemsChildAloneBecomesTopLevel()
{
    Note n = parse(uR"(- [ ] Parent <!--n id:A-->
  - [ ] Child <!--n id:B-->
)"_s);

    const QList<Item> taken = n.takeItems({u"B"_s});
    QCOMPARE(taken.size(), 1);
    QCOMPARE(taken.first().depth, 0);
    QCOMPARE(n.items.size(), 1);
}

void MdNoteTest::takeItemsUnknownIdIsANoOp()
{
    Note n = parse(u"- [ ] One <!--n id:A-->\n"_s);
    QVERIFY(n.takeItems({u"missing"_s}).isEmpty());
    QCOMPARE(n.items.size(), 1);
}

void MdNoteTest::appendItemsGoesToTheEnd()
{
    Note n = parse(u"- [ ] One <!--n id:A-->\n\n## Notes\n\nProse.\n"_s);

    Item extra;
    extra.id = u"B"_s;
    extra.text = u"Two"_s;
    n.appendItems({extra});

    QCOMPARE(n.items.size(), 2);
    QCOMPARE(n.items.last().id, u"B"_s);
    QVERIFY(n.body.contains(u"Prose."_s));
}

void MdNoteTest::headingsBetweenItemsAreGroupHeadings()
{
    const QString src = read(QStringLiteral(NOTA_TEST_DATA_DIR "/headings.md"));
    const Note n = parse(src);

    QVERIFY(n.items.size() >= 4);
    const Item *must = nullptr;
    for (const Item &it : n.items) {
        if (it.isHeading() && it.text == u"Must"_s) {
            must = &it;
        }
    }
    QVERIFY(must != nullptr);
    QCOMPARE(must->level, 2);
    // "## Notes", which leads into prose, stays body rather than becoming one.
    QVERIFY(n.body.contains(u"## Notes"_s));
    QCOMPARE(serialize(n), src);
}

void MdNoteTest::headingFollowedByProseIsBody()
{
    const Note n = parse(u"- [ ] One <!--n id:A-->\n\n## Notes\n\nProse only.\n"_s);

    QCOMPARE(n.items.size(), 1);
    QVERIFY(!n.items.first().isHeading());
    QVERIFY(n.body.startsWith(u"## Notes"_s));
}

void MdNoteTest::consecutiveHeadingsBeforeAnItem()
{
    const Note n = parse(u"## Must\n\n### Today\n\n- [ ] Ship it <!--n id:A-->\n"_s);

    QCOMPARE(n.items.size(), 3);
    QVERIFY(n.items.at(0).isHeading());
    QCOMPARE(n.items.at(0).level, 2);
    QVERIFY(n.items.at(1).isHeading());
    QCOMPARE(n.items.at(1).level, 3);
    QVERIFY(!n.items.at(2).isHeading());
}

void MdNoteTest::headingsCarryNoLabelsAndAreNeverTaken()
{
    Note n = parse(u"## Must #notalabel\n\n- [ ] Ship it #ops <!--n id:A-->\n"_s);

    QCOMPARE(n.allLabels(), QStringList({u"ops"_s}));
    QVERIFY(n.takeItems({u"A"_s}).size() == 1);
    // The heading is left behind rather than travelling with the item.
    QCOMPARE(n.items.size(), 1);
    QVERIFY(n.items.first().isHeading());
}

QTEST_GUILESS_MAIN(MdNoteTest)

#include "mdnotetest.moc"
