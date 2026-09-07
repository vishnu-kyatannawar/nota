/*
 * SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
 * SPDX-License-Identifier: MIT
 */

#include "vault.h"
#include "workplan.h"

#include <QSignalSpy>
#include <QTemporaryDir>
#include <QTest>

using namespace Qt::StringLiterals;

namespace
{
constexpr QLatin1StringView Folder{"Workplans"};

/*! Ids that a test can name, so a golden file has something to compare to. */
Workplan::IdGenerator countingIds(std::shared_ptr<int> counter)
{
    return [counter] {
        return QStringLiteral("SEED%1").arg((*counter)++);
    };
}
} // namespace

class WorkplanTest : public QObject
{
    Q_OBJECT

private Q_SLOTS:
    void init();
    void cleanup();

    void ensureCreatesADatedNoteWithFrontmatter();
    void ensureNeverTouchesADayThatAlreadyExists();
    void unfinishedItemsCarryKeepingTheirIdentity();
    void finishedItemsStayOnTheDayTheyWereFinished();
    void mondayCarriesFromFriday();
    void aHeadingCarriesOnlyWhenSomethingUnderItDoes();
    void carriedCountsUpAndFromIsSetOnlyOnce();
    void weekendsCanBeSwitchedOff();

    void cadencesDecideWhichDaysAreDue();
    void aRepeatingItemComesBackWhetherOrNotItWasDone();
    void aRepeatingItemIsNeverCarried();
    void addTemplateMintsAnIdThatSurvivesRenaming();
    void addingARepeatReachesTodayNotJustTomorrow();
    void stoppingARepeatLeavesEarlierWorkplansExactlyAsTheyWere();
    void handEditedTemplatesGainIdsOnFirstRead();

    void hoursAndDayTypeAreValidated();
    void suggestedMinutesSumsTheItemLogs();

private:
    QString read(const QString &rel) const;
    void write(const QString &rel, const QString &content) const;
    Workplan::Manager manager(bool createOnWeekends = true);

    std::unique_ptr<QTemporaryDir> m_dir;
    std::unique_ptr<Vault> m_vault;
    std::shared_ptr<int> m_counter;
};

void WorkplanTest::init()
{
    m_dir = std::make_unique<QTemporaryDir>();
    QVERIFY(m_dir->isValid());
    m_vault = std::make_unique<Vault>(m_dir->path());
    QVERIFY(m_vault->isOpen());
    m_counter = std::make_shared<int>(1);
}

void WorkplanTest::cleanup()
{
    m_vault.reset();
    m_dir.reset();
}

Workplan::Manager WorkplanTest::manager(bool createOnWeekends)
{
    Workplan::Manager::Options options;
    options.folder = QString(Folder);
    options.createOnWeekends = createOnWeekends;
    options.newId = countingIds(m_counter);
    return Workplan::Manager(m_vault.get(), options);
}

QString WorkplanTest::read(const QString &rel) const
{
    const auto raw = m_vault->readRaw(rel);
    return raw.value_or(QString());
}

void WorkplanTest::write(const QString &rel, const QString &content) const
{
    QVERIFY(m_vault->writeRaw(rel, content));
}

void WorkplanTest::ensureCreatesADatedNoteWithFrontmatter()
{
    auto m = manager();
    const QDate day(2026, 9, 2); // a Wednesday

    const QString path = m.ensure(day);
    QCOMPARE(path, u"Workplans/2026-09-02.md"_s);
    QCOMPARE(read(path), uR"(---
type: workplan
date: 2026-09-02
hours: "00:00"
daytype: work
---
)"_s);
}

void WorkplanTest::ensureNeverTouchesADayThatAlreadyExists()
{
    auto m = manager();
    const QDate day(2026, 9, 2);

    m.ensure(day);
    write(m.pathFor(day), u"---\ntype: workplan\ndate: 2026-09-02\n---\n\n- [ ] Mine <!--n id:MINE-->\n"_s);

    // ensure() runs on launch, at midnight and on window focus.
    for (int i = 0; i < 3; ++i) {
        QCOMPARE(m.ensure(day), m.pathFor(day));
    }
    QVERIFY(read(m.pathFor(day)).contains(u"id:MINE"_s));
}

void WorkplanTest::unfinishedItemsCarryKeepingTheirIdentity()
{
    auto m = manager();
    write(u"Workplans/2026-09-01.md"_s,
          uR"(---
type: workplan
date: 2026-09-01
hours: "07:30"
daytype: work
---

- [ ] Review PR 412 #rv-portal [00:45] <!--n id:KEEP t:09:40-->
      A body line that must travel too.
  - [ ] Check the migration <!--n id:CHILD t:09:41-->
)"_s);

    m.ensure(QDate(2026, 9, 2));
    const QString today = read(u"Workplans/2026-09-02.md"_s);

    // Id, creation time, labels, logged time, body and nesting all survive.
    QVERIFY(today.contains(u"id:KEEP"_s));
    QVERIFY(today.contains(u"t:09:40"_s));
    QVERIFY(today.contains(u"#rv-portal [00:45]"_s));
    QVERIFY(today.contains(u"A body line that must travel too."_s));
    QVERIFY(today.contains(u"  - [ ] Check the migration"_s));
    // Only the carry bookkeeping is new; the day's own hours are not inherited.
    QVERIFY(today.contains(u"from:2026-09-01 carried:1"_s));
    QVERIFY(today.contains(u"hours: \"00:00\""_s));
}

void WorkplanTest::finishedItemsStayOnTheDayTheyWereFinished()
{
    auto m = manager();
    write(u"Workplans/2026-09-01.md"_s,
          uR"(---
type: workplan
date: 2026-09-01
---

- [x] Done yesterday <!--n id:DONE t:09:00 done:11:02-->
- [ ] Still open <!--n id:OPEN t:09:01-->
)"_s);

    m.ensure(QDate(2026, 9, 2));
    const QString today = read(u"Workplans/2026-09-02.md"_s);

    QVERIFY(!today.contains(u"id:DONE"_s));
    QVERIFY(today.contains(u"id:OPEN"_s));
    // Yesterday is a record of yesterday and is left alone.
    QVERIFY(read(u"Workplans/2026-09-01.md"_s).contains(u"done:11:02"_s));
}

void WorkplanTest::mondayCarriesFromFriday()
{
    // Weekend notes off, so Saturday and Sunday leave no file behind at all.
    auto m = manager(false);
    write(u"Workplans/2026-09-04.md"_s, // Friday
          u"---\ntype: workplan\ndate: 2026-09-04\n---\n\n- [ ] From Friday <!--n id:FRI t:16:00-->\n"_s);

    m.ensure(QDate(2026, 9, 7)); // Monday
    const QString monday = read(u"Workplans/2026-09-07.md"_s);

    // The rule is "the most recent workplan", not "yesterday", so a weekend,
    // a week of leave or a holiday never breaks the chain.
    QVERIFY(monday.contains(u"id:FRI"_s));
    QVERIFY(monday.contains(u"from:2026-09-04 carried:1"_s));
}

void WorkplanTest::aHeadingCarriesOnlyWhenSomethingUnderItDoes()
{
    auto m = manager();
    write(u"Workplans/2026-09-01.md"_s,
          uR"(---
type: workplan
date: 2026-09-01
---

## Finished

- [x] All done <!--n id:A t:09:00 done:10:00-->

## Must

- [ ] Not yet <!--n id:B t:09:01-->
)"_s);

    m.ensure(QDate(2026, 9, 2));
    const QString today = read(u"Workplans/2026-09-02.md"_s);

    QVERIFY2(!today.contains(u"## Finished"_s), "a group that finished vanishes with its heading");
    QVERIFY(today.contains(u"## Must"_s));
    QVERIFY(today.contains(u"id:B"_s));
}

void WorkplanTest::carriedCountsUpAndFromIsSetOnlyOnce()
{
    auto m = manager();
    write(u"Workplans/2026-09-01.md"_s,
          u"---\ntype: workplan\ndate: 2026-09-01\n---\n\n- [ ] Chase it <!--n id:C t:09:00-->\n"_s);

    m.ensure(QDate(2026, 9, 2));
    QVERIFY(read(u"Workplans/2026-09-02.md"_s).contains(u"from:2026-09-01 carried:1"_s));

    m.ensure(QDate(2026, 9, 3));
    // "from" still names the day it was born, not the day it last moved.
    QVERIFY(read(u"Workplans/2026-09-03.md"_s).contains(u"from:2026-09-01 carried:2"_s));
}

void WorkplanTest::weekendsCanBeSwitchedOff()
{
    auto off = manager(false);
    QVERIFY(off.ensure(QDate(2026, 9, 5)).isEmpty()); // Saturday
    QVERIFY(off.ensure(QDate(2026, 9, 6)).isEmpty()); // Sunday
    QCOMPARE(off.ensure(QDate(2026, 9, 7)), u"Workplans/2026-09-07.md"_s);

    auto on = manager(true);
    QCOMPARE(on.ensure(QDate(2026, 9, 5)), u"Workplans/2026-09-05.md"_s);
    QVERIFY(read(u"Workplans/2026-09-05.md"_s).contains(u"daytype: weekend"_s));
}

void WorkplanTest::cadencesDecideWhichDaysAreDue()
{
    const QDate saturday(2026, 9, 5);
    const QDate monday(2026, 9, 7);
    const QDate friday(2026, 9, 11);

    const auto due = [](const QString &cadence, QDate day) {
        return Workplan::Template{QStringLiteral("id"), QStringLiteral("text"), cadence}.dueOn(day);
    };

    QVERIFY(due(u"daily"_s, saturday));
    QVERIFY(!due(u"weekdays"_s, saturday));
    QVERIFY(due(u"weekdays"_s, monday));
    QVERIFY(due(u"weekly:fri"_s, friday));
    QVERIFY(!due(u"weekly:fri"_s, monday));
    QVERIFY(!due(u"nonsense"_s, monday));
}

void WorkplanTest::aRepeatingItemComesBackWhetherOrNotItWasDone()
{
    auto m = manager();
    write(QString(Workplan::TemplatePath), u"- [ ] Check calendar #daily @daily <!--n rec:cal-->\n"_s);
    write(u"Workplans/2026-09-01.md"_s,
          u"---\ntype: workplan\ndate: 2026-09-01\n---\n\n"
          u"- [x] Check calendar #daily <!--n id:OLD t:09:00 done:09:05 rec:cal-->\n"_s);

    m.ensure(QDate(2026, 9, 2));
    const QString today = read(u"Workplans/2026-09-02.md"_s);

    QVERIFY(today.contains(u"- [ ] Check calendar #daily"_s));
    QVERIFY(today.contains(u"rec:cal"_s));
    // Fresh, so neither yesterday's tick nor a carry badge comes with it.
    QVERIFY(!today.contains(u"done:"_s));
    QVERIFY(!today.contains(u"carried:"_s));
    QVERIFY(!today.contains(u"id:OLD"_s));
}

void WorkplanTest::aRepeatingItemIsNeverCarried()
{
    auto m = manager();
    // The template file is gone, so nothing can seed it back.
    write(u"Workplans/2026-09-01.md"_s,
          u"---\ntype: workplan\ndate: 2026-09-01\n---\n\n"
          u"- [ ] Was repeating <!--n id:OLD t:09:00 rec:gone-->\n"_s);

    m.ensure(QDate(2026, 9, 2));
    QVERIFY(!read(u"Workplans/2026-09-02.md"_s).contains(u"rec:gone"_s));
}

void WorkplanTest::addTemplateMintsAnIdThatSurvivesRenaming()
{
    auto m = manager();

    Workplan::Template added;
    QVERIFY(m.addTemplate(u"Log the day bill"_s, &added));
    QCOMPARE(added.cadence, u"daily"_s);
    QVERIFY(!added.id.isEmpty());

    QVERIFY(m.renameTemplate(added.id, u"Log the daily bill"_s));

    const QList<Workplan::Template> all = m.templates();
    QCOMPARE(all.size(), 1);
    // Renaming must not turn it into a different item, which would seed a
    // duplicate alongside the original.
    QCOMPARE(all.first().id, added.id);
    QCOMPARE(all.first().text, u"Log the daily bill"_s);
}

void WorkplanTest::addingARepeatReachesTodayNotJustTomorrow()
{
    auto m = manager();
    const QDate today(2026, 9, 2);
    m.ensure(today);

    Workplan::Template added;
    QVERIFY(m.addTemplate(u"Stand-up"_s, &added));
    QVERIFY(m.seedInto(today));

    const QString file = read(m.pathFor(today));
    QVERIFY(file.contains(u"- [ ] Stand-up"_s));
    QVERIFY(file.contains(u"rec:"_s + added.id));

    // Seeding twice must not double it.
    QVERIFY(m.seedInto(today));
    QCOMPARE(read(m.pathFor(today)).count(u"Stand-up"_s), 1);
}

void WorkplanTest::stoppingARepeatLeavesEarlierWorkplansExactlyAsTheyWere()
{
    auto m = manager();
    const QDate today(2026, 9, 2);

    write(u"Workplans/2026-09-01.md"_s,
          u"---\ntype: workplan\ndate: 2026-09-01\n---\n\n"
          u"- [x] Stand-up <!--n id:YDAY t:09:00 done:09:15 rec:standup-->\n"_s);
    const QString yesterdayBefore = read(u"Workplans/2026-09-01.md"_s);

    write(QString(Workplan::TemplatePath), u"- [ ] Stand-up <!--n rec:standup-->\n"_s);
    m.ensure(today);
    QVERIFY(read(m.pathFor(today)).contains(u"rec:standup"_s));

    QVERIFY(m.removeTemplate(u"standup"_s));
    QVERIFY(m.dropFrom(today, u"standup"_s));

    QVERIFY(!read(m.pathFor(today)).contains(u"rec:standup"_s));
    // What you did on a day is a record of that day.
    QCOMPARE(read(u"Workplans/2026-09-01.md"_s), yesterdayBefore);

    // Tomorrow has nothing left to seed from.
    m.ensure(QDate(2026, 9, 3));
    QVERIFY(!read(u"Workplans/2026-09-03.md"_s).contains(u"rec:standup"_s));
}

void WorkplanTest::handEditedTemplatesGainIdsOnFirstRead()
{
    auto m = manager();
    write(QString(Workplan::TemplatePath),
          u"- [ ] Check calendar #daily @daily\n"
          u"- [ ] Log the day bill #billing @weekdays\n"
          u"- [ ] Weekly report @weekly:fri\n"_s);

    const QList<Workplan::Template> all = m.templates();
    QCOMPARE(all.size(), 3);
    QCOMPARE(all.at(0).cadence, u"daily"_s);
    QCOMPARE(all.at(1).cadence, u"weekdays"_s);
    QCOMPARE(all.at(2).cadence, u"weekly:fri"_s);
    // The cadence token is stripped from the text but kept in the file.
    QCOMPARE(all.at(2).text, u"Weekly report"_s);
    QCOMPARE(all.at(0).id, u"check-calendar-daily"_s);

    QVERIFY(read(QString(Workplan::TemplatePath)).contains(u"rec:check-calendar-daily"_s));
    QVERIFY(read(QString(Workplan::TemplatePath)).contains(u"@weekly:fri"_s));
}

void WorkplanTest::hoursAndDayTypeAreValidated()
{
    auto m = manager();
    const QString path = m.ensure(QDate(2026, 9, 2));

    QVERIFY(m.setHours(path, u"07:30"_s));
    QVERIFY(read(path).contains(u"hours: \"07:30\""_s));
    QVERIFY(!m.setHours(path, u"7:30"_s));
    QVERIFY(!m.setHours(path, u"nonsense"_s));
    QVERIFY(read(path).contains(u"hours: \"07:30\""_s));

    QVERIFY(m.setDayType(path, u"leave"_s));
    QVERIFY(read(path).contains(u"daytype: leave"_s));
    QVERIFY(!m.setDayType(path, u"holiday-ish"_s));
    // Marking a day off does not erase time already logged against it.
    QVERIFY(read(path).contains(u"hours: \"07:30\""_s));
}

void WorkplanTest::suggestedMinutesSumsTheItemLogs()
{
    auto m = manager();
    write(u"Workplans/2026-09-02.md"_s,
          u"---\ntype: workplan\ndate: 2026-09-02\n---\n\n"
          u"- [x] One [01:20] <!--n id:A-->\n"
          u"- [ ] Two [00:40] <!--n id:B-->\n"
          u"- [ ] Three <!--n id:C-->\n"_s);

    QCOMPARE(m.suggestedMinutes(u"Workplans/2026-09-02.md"_s), 120);
}

QTEST_GUILESS_MAIN(WorkplanTest)

#include "workplantest.moc"
