/*
 * SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
 * SPDX-License-Identifier: MIT
 *
 * The save pipeline, from an edit in the model to the bytes on disk.
 *
 * These drive Nota itself rather than the model, because the bugs worth
 * catching here are about timing and identity — a debounce firing after the
 * page changed, a completion stamp written on the wrong transition — and none
 * of them show up in a model-only test.
 */

#include "app.h"
#include "itemmodel.h"
#include "mdnote.h"
#include "settings.h"

#include <QFile>
#include <QSignalSpy>
#include <QTemporaryDir>
#include <QTest>

using namespace Qt::StringLiterals;

class SaveTest : public QObject
{
    Q_OBJECT

private Q_SLOTS:
    void init();
    void cleanup();

    void anEditReachesDiskAfterTheDebounce();
    void flushWritesImmediately();
    void switchingPageWritesTheOldPageNotTheNew();
    void tickingStampsTheCompletionTimeAndUntickingClearsIt();
    void savingKeepsRolloverMetadataAndTheTrailingProse();
    void aNewRowGetsAnIdWhenItIsSaved();
    void theBodyIsSavedSeparatelyFromTheItems();

private:
    QString read(const QString &rel) const;
    void write(const QString &rel, const QString &content) const;
    void waitForSave() const;

    std::unique_ptr<QTemporaryDir> m_dir;
    std::unique_ptr<Nota> m_app;
};

void SaveTest::init()
{
    m_dir = std::make_unique<QTemporaryDir>();
    QVERIFY(m_dir->isValid());

    // Nota resolves its vault from settings, so point the startup override at
    // the temporary one rather than touching the developer's real notes.
    Nota::setStartupVault(m_dir->path());
    m_app = std::make_unique<Nota>();
    QVERIFY(m_app->errorMessage().isEmpty());
}

void SaveTest::cleanup()
{
    m_app.reset();
    Nota::setStartupVault(QString());
    m_dir.reset();
}

QString SaveTest::read(const QString &rel) const
{
    QFile f(m_dir->path() + u'/' + rel);
    if (!f.open(QIODevice::ReadOnly)) {
        return {};
    }
    return QString::fromUtf8(f.readAll());
}

void SaveTest::write(const QString &rel, const QString &content) const
{
    const QString path = m_dir->path() + u'/' + rel;
    QDir().mkpath(QFileInfo(path).absolutePath());
    QFile f(path);
    QVERIFY(f.open(QIODevice::WriteOnly));
    f.write(content.toUtf8());
}

void SaveTest::waitForSave() const
{
    // Longer than the 400 ms item debounce, so a real timer failure shows up
    // as a failure rather than as a flake.
    QTRY_VERIFY_WITH_TIMEOUT(!m_app->isDirty(), 3000);
}

void SaveTest::anEditReachesDiskAfterTheDebounce()
{
    // Launch already created and opened today's workplan.
    const QString path = m_app->currentPath();
    QVERIFY(!path.isEmpty());

    m_app->items()->insertItem(-1, 0);
    m_app->items()->setText(0, u"Written by the test"_s);
    QVERIFY(m_app->isDirty());

    waitForSave();
    QVERIFY(read(path).contains(u"- [ ] Written by the test"_s));
}

void SaveTest::flushWritesImmediately()
{
    const QString path = m_app->currentPath();
    m_app->items()->insertItem(-1, 0);
    m_app->items()->setText(0, u"Urgent"_s);

    m_app->flush();

    // No waiting: quitting or switching away must never lose the last edit.
    QVERIFY(!m_app->isDirty());
    QVERIFY(read(path).contains(u"Urgent"_s));
}

void SaveTest::switchingPageWritesTheOldPageNotTheNew()
{
    write(u"Other.md"_s, u"- [ ] Belongs to Other <!--n id:OTHER-->\n"_s);

    const QString workplan = m_app->currentPath();
    m_app->items()->insertItem(-1, 0);
    m_app->items()->setText(0, u"Belongs to the workplan"_s);
    QVERIFY(m_app->isDirty());

    // Switching while a save is pending. The armed save belongs to the page
    // being left, and must not follow the user to the new one.
    m_app->open(u"Other.md"_s);
    waitForSave();

    QVERIFY(read(workplan).contains(u"Belongs to the workplan"_s));
    QVERIFY2(!read(u"Other.md"_s).contains(u"Belongs to the workplan"_s),
             "a pending save must never write one page's items into another");
    QVERIFY(read(u"Other.md"_s).contains(u"Belongs to Other"_s));
}

void SaveTest::tickingStampsTheCompletionTimeAndUntickingClearsIt()
{
    const QString path = m_app->currentPath();
    m_app->items()->insertItem(-1, 0);
    m_app->items()->setText(0, u"Tick me"_s);
    m_app->flush();

    m_app->items()->toggleDone(0);
    m_app->flush();
    QVERIFY(read(path).contains(u"- [x] Tick me"_s));
    QVERIFY2(read(path).contains(u"done:"_s), "ticking stamps when it was finished");

    m_app->items()->toggleDone(0);
    m_app->flush();
    QVERIFY(read(path).contains(u"- [ ] Tick me"_s));
    // A reopened item must not go on claiming it was finished.
    QVERIFY(!read(path).contains(u"done:"_s));
}

void SaveTest::savingKeepsRolloverMetadataAndTheTrailingProse()
{
    write(u"Page.md"_s,
          u"---\nlayout: items\n---\n\n"
          u"- [ ] Carried thing <!--n id:KEEP t:09:40 from:2026-09-01 carried:3 rec:standup-->\n"
          u"\n## Notes\n\nProse that must survive.\n"_s);
    m_app->open(u"Page.md"_s);
    QCOMPARE(m_app->items()->rowCount(), 1);

    m_app->items()->setText(0, u"Carried thing, reworded"_s);
    m_app->flush();

    const QString saved = read(u"Page.md"_s);
    QVERIFY(saved.contains(u"Carried thing, reworded"_s));
    // The editor never sees any of this, so it is looked up by id and kept.
    QVERIFY(saved.contains(u"id:KEEP"_s));
    QVERIFY(saved.contains(u"t:09:40"_s));
    QVERIFY(saved.contains(u"from:2026-09-01"_s));
    QVERIFY(saved.contains(u"carried:3"_s));
    QVERIFY(saved.contains(u"rec:standup"_s));
    QVERIFY(saved.contains(u"layout: items"_s));
    QVERIFY(saved.contains(u"Prose that must survive."_s));
}

void SaveTest::aNewRowGetsAnIdWhenItIsSaved()
{
    const QString path = m_app->currentPath();
    m_app->items()->insertItem(-1, 0);
    m_app->items()->setText(0, u"Brand new"_s);
    m_app->flush();

    // Without an id the editor cannot address the row to tick or delete it.
    const auto note = MdNote::parse(read(path));
    const int row = note.items.size() - 1;
    QVERIFY(row >= 0);
    QCOMPARE(note.items.at(row).text, u"Brand new"_s);
    QVERIFY(!note.items.at(row).id.isEmpty());
    QVERIFY(!note.items.at(row).createdAt.isEmpty());
}

void SaveTest::theBodyIsSavedSeparatelyFromTheItems()
{
    write(u"Prose.md"_s, u"- [ ] One <!--n id:A-->\n"_s);
    m_app->open(u"Prose.md"_s);

    m_app->setBody(u"Some prose typed underneath.\n"_s);
    QVERIFY(m_app->isDirty());
    m_app->flush();

    const QString saved = read(u"Prose.md"_s);
    QVERIFY(saved.contains(u"- [ ] One"_s));
    QVERIFY(saved.contains(u"Some prose typed underneath."_s));
}

QTEST_GUILESS_MAIN(SaveTest)

#include "savetest.moc"
